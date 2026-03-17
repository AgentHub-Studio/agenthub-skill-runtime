package dev.cezar.agenthub.skillruntime.executor;

import dev.cezar.agenthub.skillruntime.domain.Tool;
import dev.cezar.agenthub.skillruntime.multitenant.MultiTenant;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.r2dbc.core.DatabaseClient;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;
import java.util.UUID;

/**
 * Executor para busca semântica de documentos usando pgvector.
 * <p>
 * Realiza busca por similaridade vetorial (cosine distance) em embeddings de documentos.
 * Pipeline:
 * 1. Gera embedding da query via serviço de embedding
 * 2. Busca documentos similares via pgvector (cosine similarity)
 * 3. Retorna top-k documentos mais relevantes
 * </p>
 * <p>
 * Configuração esperada da tool:
 * <pre>{@code
 * {
 *   "embeddingServiceUrl": "http://agenthub-embedding:8000/embed",
 *   "embeddingModel": "e5-large",  // opcional
 *   "defaultLimit": 10             // opcional
 * }
 * }</pre>
 * </p>
 * <p>
 * Input esperado:
 * <pre>{@code
 * {
 *   "query": "texto da busca",
 *   "knowledgeBaseId": "uuid-da-kb",
 *   "limit": 5,                    // opcional (default: 10)
 *   "minScore": 0.7                // opcional (default: 0.0)
 * }
 * }</pre>
 * </p>
 *
 * @since 1.0.0
 */
@Slf4j
@Component
@AllArgsConstructor
public class DocumentSearchToolExecutor implements ToolExecutor {

    private final DatabaseClient databaseClient;
    private final WebClient.Builder webClientBuilder;

    @Override
    public String getSupportedType() {
        return "DOCUMENT_SEARCH";
    }

    @Override
    public Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context) {
        log.info("Executing document search: toolId={}, query={}", 
                tool.id(), input.get("query"));

        String query = (String) input.get("query");
        UUID knowledgeBaseId = UUID.fromString((String) input.get("knowledgeBaseId"));
        int limit = input.containsKey("limit") ? (int) input.get("limit") : 10;
        double minScore = input.containsKey("minScore") ? (double) input.get("minScore") : 0.0;

        return generateEmbedding(tool, query)
                .flatMap(queryEmbedding -> searchSimilarDocuments(
                        context.tenantId(),
                        knowledgeBaseId,
                        queryEmbedding,
                        limit,
                        minScore
                ))
                .map(documents -> {
                    log.info("Document search completed: toolId={}, found={} documents", 
                            tool.id(), documents.size());
                    return Map.of(
                            "status", "success",
                            "query", query,
                            "documents", documents,
                            "count", documents.size()
                    );
                })
                .onErrorResume(error -> {
                    log.error("Document search failed: toolId={}, error={}", 
                            tool.id(), error.getMessage());
                    return Mono.just(Map.of(
                            "status", "error",
                            "error", error.getMessage(),
                            "query", query
                    ));
                });
    }

    @Override
    public Mono<Void> validate(Tool tool, Map<String, Object> input) {
        // Valida query
        if (!input.containsKey("query") || input.get("query") == null) {
            return Mono.error(new IllegalArgumentException("Document search requires 'query' parameter"));
        }

        // Valida knowledgeBaseId
        if (!input.containsKey("knowledgeBaseId") || input.get("knowledgeBaseId") == null) {
            return Mono.error(new IllegalArgumentException("Document search requires 'knowledgeBaseId' parameter"));
        }

        // Valida embedding service URL
        String embeddingUrl = (String) tool.config().get("embeddingServiceUrl");
        if (embeddingUrl == null || embeddingUrl.isBlank()) {
            return Mono.error(new IllegalArgumentException(
                    "Document search tool requires 'embeddingServiceUrl' in config"));
        }

        return Mono.empty();
    }

    /**
     * Gera embedding da query usando serviço de embedding.
     */
    private Mono<float[]> generateEmbedding(Tool tool, String text) {
        String embeddingUrl = (String) tool.config().get("embeddingServiceUrl");
        
        WebClient client = webClientBuilder.baseUrl(embeddingUrl).build();

        return client.post()
                .bodyValue(Map.of("text", text))
                .retrieve()
                .bodyToMono(Map.class)
                .map(response -> {
                    List<Double> embedding = (List<Double>) response.get("embedding");
                    float[] result = new float[embedding.size()];
                    for (int i = 0; i < embedding.size(); i++) {
                        result[i] = embedding.get(i).floatValue();
                    }
                    log.debug("Generated embedding: dimension={}", result.length);
                    return result;
                });
    }

    /**
     * Busca documentos similares usando pgvector cosine similarity.
     */
    private Mono<List<Map<String, Object>>> searchSimilarDocuments(
            String tenantId,
            UUID knowledgeBaseId,
            float[] queryEmbedding,
            int limit,
            double minScore) {

        // Converte array para formato pgvector
        String vectorStr = formatVector(queryEmbedding);

        // Schema do tenant (UUID hyphens must be preserved)
        String schema = MultiTenant.SCHEMA_PREFIX + tenantId;

        // Query pgvector com cosine similarity
        String sql = String.format("""
            SELECT 
                dc.id as chunk_id,
                dc.text,
                dc.chunk_index,
                dc.metadata_json,
                d.id as document_id,
                d.file_name,
                1 - (dce.embedding <=> '%s'::vector) as similarity_score
            FROM %s.document_chunk_embedding dce
            JOIN %s.document_chunk dc ON dce.document_chunk_id = dc.id
            JOIN %s.document d ON dc.document_id = d.id
            WHERE dce.knowledge_base_id = :knowledgeBaseId
              AND 1 - (dce.embedding <=> '%s'::vector) >= :minScore
            ORDER BY dce.embedding <=> '%s'::vector
            LIMIT :limit
            """, vectorStr, schema, schema, schema, vectorStr, vectorStr);

        return databaseClient.sql(sql)
                .bind("knowledgeBaseId", knowledgeBaseId)
                .bind("minScore", minScore)
                .bind("limit", limit)
                .fetch()
                .all()
                .collectList();
    }

    /**
     * Formata array de floats para string pgvector: [0.1,0.2,0.3]
     */
    private String formatVector(float[] vector) {
        StringBuilder sb = new StringBuilder("[");
        for (int i = 0; i < vector.length; i++) {
            if (i > 0) sb.append(",");
            sb.append(vector[i]);
        }
        sb.append("]");
        return sb.toString();
    }
}
