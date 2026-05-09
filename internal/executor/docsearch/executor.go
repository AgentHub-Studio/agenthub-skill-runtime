package docsearch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// DocumentSearchToolExecutor searches a knowledge base using pgvector similarity search.
//
// Config fields:
//   - knowledge_base_id: string (UUID)
//   - top_k: int (default 5)
//   - similarity_threshold: float64 (default 0.7)
//   - embedding_url: string (URL of the agenthub-embedding service; falls
//     back to the executor's defaultEmbeddingURL when unset — bug 219)
type DocumentSearchToolExecutor struct {
	pool                 *pgxpool.Pool
	defaultEmbeddingURL  string
}

// NewDocumentSearchToolExecutor creates a DocumentSearchToolExecutor backed by pool.
func NewDocumentSearchToolExecutor(pool *pgxpool.Pool) *DocumentSearchToolExecutor {
	return &DocumentSearchToolExecutor{pool: pool}
}

// WithDefaultEmbeddingURL sets the fallback embedding service URL used when
// the per-tool config does not provide one (bug 219).
func (e *DocumentSearchToolExecutor) WithDefaultEmbeddingURL(url string) *DocumentSearchToolExecutor {
	e.defaultEmbeddingURL = url
	return e
}

// GetToolType returns the tool type identifier.
func (e *DocumentSearchToolExecutor) GetToolType() string { return "DOCUMENT_SEARCH" }

// docSearchConfig holds parsed configuration for a document search tool.
type docSearchConfig struct {
	KnowledgeBaseID     string  `json:"knowledge_base_id"`
	TopK                int     `json:"top_k"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	EmbeddingURL        string  `json:"embedding_url"`
}

// searchResult represents a single matched chunk returned to the caller.
type searchResult struct {
	Content      string  `json:"content"`
	Score        float64 `json:"score"`
	DocumentID   string  `json:"documentId"`
	DocumentName string  `json:"documentName"`
}

// Execute queries the embedding service and searches the knowledge base with pgvector.
func (e *DocumentSearchToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	cfg, err := parseDocSearchConfig(ec.Config)
	if err != nil {
		return nil, fmt.Errorf("docsearch executor: parse config: %w", err)
	}

	if cfg.KnowledgeBaseID == "" {
		return nil, fmt.Errorf("docsearch executor: knowledge_base_id is required")
	}
	if cfg.EmbeddingURL == "" {
		// Bug 219: fallback para o EMBEDDING_URL injetado no servidor.
		cfg.EmbeddingURL = e.defaultEmbeddingURL
	}
	if cfg.EmbeddingURL == "" {
		return nil, fmt.Errorf("docsearch executor: embedding_url is required")
	}

	query, ok := ec.Input["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("docsearch executor: input.query must be a non-empty string")
	}

	topK := cfg.TopK
	if topK <= 0 {
		topK = 5
	}
	threshold := cfg.SimilarityThreshold
	if threshold <= 0 {
		threshold = 0.7
	}

	embClient := NewEmbeddingClient(cfg.EmbeddingURL)
	embedding, err := embClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("docsearch executor: embed query: %w", err)
	}

	results, err := e.searchChunks(ctx, ec.TenantID, cfg.KnowledgeBaseID, embedding, threshold, topK)
	if err != nil {
		return nil, fmt.Errorf("docsearch executor: vector search: %w", err)
	}

	return &executor.Result{Output: map[string]any{"results": results}}, nil
}

// searchChunks performs the pgvector similarity search and returns matched chunks.
func (e *DocumentSearchToolExecutor) searchChunks(
	ctx context.Context,
	tenantID, knowledgeBaseID string,
	embedding []float32,
	threshold float64,
	topK int,
) ([]searchResult, error) {
	schema := executor.TenantSchema(tenantID)

	// pgvector expects the embedding as a vector literal: '[0.1,0.2,...]'
	vectorLiteral := float32SliceToVector(embedding)

	sql := fmt.Sprintf(`
		SELECT dc.content, d.file_name, d.id::text, 1 - (dce.embedding <=> $1::vector) AS score
		FROM %s.document_chunk dc
		JOIN %s.document_chunk_embedding dce ON dce.chunk_id = dc.id
		JOIN %s.document d ON d.id = dc.document_id
		WHERE d.knowledge_base_id = $2
		  AND 1 - (dce.embedding <=> $1::vector) >= $3
		ORDER BY score DESC
		LIMIT $4
	`, schema, schema, schema)

	rows, err := e.pool.Query(ctx, sql, vectorLiteral, knowledgeBaseID, threshold, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search query: %w", err)
	}
	defer rows.Close()

	var results []searchResult
	for rows.Next() {
		var r searchResult
		if err := rows.Scan(&r.Content, &r.DocumentName, &r.DocumentID, &r.Score); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// float32SliceToVector converts a []float32 to a pgvector literal string.
func float32SliceToVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	sb := make([]byte, 0, len(v)*10+2)
	sb = append(sb, '[')
	for i, f := range v {
		if i > 0 {
			sb = append(sb, ',')
		}
		sb = append(sb, fmt.Sprintf("%g", f)...)
	}
	sb = append(sb, ']')
	return string(sb)
}

// parseDocSearchConfig decodes the executor config map.
//
// Bug 218: aceita tanto snake_case (knowledge_base_id, top_k,
// similarity_threshold, embedding_url) quanto camelCase (kbId, topK,
// similarityThreshold, embeddingUrl) — frontend e backend Java usavam
// camelCase enquanto este executor sempre exigiu snake_case.
func parseDocSearchConfig(raw map[string]any) (*docSearchConfig, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var cfg docSearchConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	// camelCase fallbacks (frontend/Java legacy)
	if cfg.KnowledgeBaseID == "" {
		if v, ok := raw["kbId"].(string); ok {
			cfg.KnowledgeBaseID = v
		}
	}
	if cfg.TopK == 0 {
		if v, ok := raw["topK"].(float64); ok {
			cfg.TopK = int(v)
		} else if v, ok := raw["limit"].(float64); ok {
			cfg.TopK = int(v)
		}
	}
	if cfg.SimilarityThreshold == 0 {
		if v, ok := raw["similarityThreshold"].(float64); ok {
			cfg.SimilarityThreshold = v
		}
	}
	if cfg.EmbeddingURL == "" {
		if v, ok := raw["embeddingUrl"].(string); ok {
			cfg.EmbeddingURL = v
		}
	}
	return &cfg, nil
}
