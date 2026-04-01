package docsearch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

func TestParseDocSearchConfig_Defaults(t *testing.T) {
	cfg, err := parseDocSearchConfig(map[string]any{
		"knowledge_base_id": "kb-123",
		"embedding_url":     "http://embed:8080",
	})
	require.NoError(t, err)
	assert.Equal(t, "kb-123", cfg.KnowledgeBaseID)
	assert.Equal(t, "http://embed:8080", cfg.EmbeddingURL)
	assert.Equal(t, 0, cfg.TopK)               // caller applies default
	assert.Equal(t, 0.0, cfg.SimilarityThreshold) // caller applies default
}

func TestParseDocSearchConfig_AllFields(t *testing.T) {
	cfg, err := parseDocSearchConfig(map[string]any{
		"knowledge_base_id":     "kb-abc",
		"embedding_url":         "http://embed:9090",
		"top_k":                 10,
		"similarity_threshold":  0.85,
	})
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.TopK)
	assert.InDelta(t, 0.85, cfg.SimilarityThreshold, 1e-9)
}

func TestExecute_MissingKnowledgeBaseID(t *testing.T) {
	exec := NewDocumentSearchToolExecutor(nil)
	ec := executor.ExecutionContext{
		Config: map[string]any{
			"embedding_url": "http://embed:8080",
		},
		Input: map[string]any{"query": "test"},
	}
	_, err := exec.Execute(context.Background(), ec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knowledge_base_id is required")
}

func TestExecute_MissingEmbeddingURL(t *testing.T) {
	exec := NewDocumentSearchToolExecutor(nil)
	ec := executor.ExecutionContext{
		Config: map[string]any{
			"knowledge_base_id": "kb-123",
		},
		Input: map[string]any{"query": "test"},
	}
	_, err := exec.Execute(context.Background(), ec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding_url is required")
}

func TestExecute_MissingQuery(t *testing.T) {
	exec := NewDocumentSearchToolExecutor(nil)
	ec := executor.ExecutionContext{
		Config: map[string]any{
			"knowledge_base_id": "kb-123",
			"embedding_url":     "http://embed:8080",
		},
		Input: map[string]any{},
	}
	_, err := exec.Execute(context.Background(), ec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input.query must be a non-empty string")
}

func TestFloat32SliceToVector_Empty(t *testing.T) {
	assert.Equal(t, "[]", float32SliceToVector(nil))
	assert.Equal(t, "[]", float32SliceToVector([]float32{}))
}

func TestFloat32SliceToVector_Values(t *testing.T) {
	result := float32SliceToVector([]float32{0.1, 0.2, 0.3})
	assert.Equal(t, "[0.1,0.2,0.3]", result)
}

func TestGetToolType(t *testing.T) {
	exec := NewDocumentSearchToolExecutor(nil)
	assert.Equal(t, "DOCUMENT_SEARCH", exec.GetToolType())
}
