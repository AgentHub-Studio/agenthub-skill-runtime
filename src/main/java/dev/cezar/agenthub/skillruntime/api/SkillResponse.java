package dev.cezar.agenthub.skillruntime.api;

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
public record SkillResponse(
        UUID executionId,
        String skillSlug,
        UUID toolId,
        boolean success,
        Map<String, Object> result,
        String error,
        Map<String, Object> metadata,
        long latencyMs,
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
