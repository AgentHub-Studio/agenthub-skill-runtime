package dev.cezar.agenthub.skillruntime.api;

import io.swagger.v3.oas.annotations.media.Schema;

import java.time.OffsetDateTime;
import java.util.Map;
import java.util.UUID;

/**
 * Response da execução de uma skill.
 *
 * @param executionId   ID único da execução
 * @param skillSlug     Slug da skill executada
 * @param toolId        ID da tool concreta que foi executada
 * @param success       Se a execução foi bem-sucedida
 * @param result        Resultado da execução (estrutura dependente da tool)
 * @param error         Mensagem de erro (se success = false)
 * @param metadata      Metadados adicionais da execução
 * @param latencyMs     Latência da execução em millisegundos
 * @param executedAt    Timestamp de execução
 * @since 1.0.0
 */
@Schema(description = "Response from skill execution with result or error details")
public record SkillResponse(
        @Schema(
            description = "Unique execution ID",
            example = "550e8400-e29b-41d4-a716-446655440000"
        )
        UUID executionId,

        @Schema(
            description = "Skill slug that was executed",
            example = "document-search"
        )
        String skillSlug,

        @Schema(
            description = "ID of the concrete tool that was executed (null if skill resolution failed)",
            example = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
        )
        UUID toolId,

        @Schema(
            description = "Whether the execution was successful",
            example = "true"
        )
        boolean success,

        @Schema(
            description = "Execution result (structure depends on tool type, validated against skill's output schema)",
            example = "{\"documents\": [{\"id\": \"doc-123\", \"content\": \"...\", \"score\": 0.95}]}"
        )
        Map<String, Object> result,

        @Schema(
            description = "Error message if execution failed (null on success)",
            example = "Tool execution failed: Connection timeout"
        )
        String error,

        @Schema(
            description = "Additional execution metadata (tool type, retry attempts, etc.)",
            example = "{\"toolType\": \"DOCUMENT_SEARCH\", \"executionTimeMs\": 245, \"attempts\": 1}"
        )
        Map<String, Object> metadata,

        @Schema(
            description = "Total execution latency in milliseconds (includes retries)",
            example = "250"
        )
        long latencyMs,

        @Schema(
            description = "Timestamp when the execution completed",
            example = "2026-03-14T10:30:00Z"
        )
        OffsetDateTime executedAt
) {
    /**
     * Cria response de sucesso.
     */
    public static SkillResponse success(
            UUID executionId,
            String skillSlug,
            UUID toolId,
            Map<String, Object> result,
            long latencyMs) {
        return new SkillResponse(
                executionId,
                skillSlug,
                toolId,
                true,
                result,
                null,
                Map.of(),
                latencyMs,
                OffsetDateTime.now()
        );
    }

    /**
     * Cria response de erro.
     */
    public static SkillResponse error(
            UUID executionId,
            String skillSlug,
            String errorMessage,
            long latencyMs) {
        return new SkillResponse(
                executionId,
                skillSlug,
                null,
                false,
                Map.of(),
                errorMessage,
                Map.of(),
                latencyMs,
                OffsetDateTime.now()
        );
    }
}
