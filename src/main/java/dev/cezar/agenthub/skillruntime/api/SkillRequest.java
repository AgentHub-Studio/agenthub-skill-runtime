package dev.cezar.agenthub.skillruntime.api;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

import java.util.Map;
import java.util.UUID;

/**
 * Request para invocar uma skill.
 * <p>
 * Skills são abstrações de alto nível que são resolvidas para tools concretas.
 * Uma skill pode ter múltiplas tools implementando a mesma funcionalidade
 * (diferentes provedores, versões, etc.).
 * </p>
 *
 * @param tenantId     ID do tenant (obrigatório para isolamento)
 * @param skillId      ID da skill (opcional, se skillSlug for fornecido)
 * @param skillSlug    Slug da skill (eg. "document-search", "sql-query")
 * @param input        Mapa de inputs para a skill (validado pelo executor)
 * @param executionContext Contexto de execução (agentId, userId, nodeId, etc.)
 * @param timeout      Timeout em millisegundos (padrão: 30000)
 * @param retryPolicy  Política de retry (padrão: null = sem retry)
 * @since 1.0.0
 */
public record SkillRequest(
        @NotNull UUID tenantId,
        UUID skillId,
        @NotBlank String skillSlug,
        @NotNull Map<String, Object> input,
        ExecutionContext executionContext,
        Integer timeout,
        RetryPolicy retryPolicy
) {
    /**
     * Contexto de execução da skill.
     *
     * @param agentId     ID do agente que está executando
     * @param userId      ID do usuário que iniciou a execução
     * @param executionId ID da execução do agente
     * @param nodeId      ID do nó do pipeline que está executando
     */
    public record ExecutionContext(
            UUID agentId,
            UUID userId,
            UUID executionId,
            String nodeId
    ) {}

    /**
     * Política de retry para a execução.
     *
     * @param maxAttempts      Número máximo de tentativas (padrão: 3)
     * @param backoffMs        Backoff inicial em ms (padrão: 1000)
     * @param backoffMultiplier Multiplicador de backoff (padrão: 2.0)
     */
    public record RetryPolicy(
            int maxAttempts,
            long backoffMs,
            double backoffMultiplier
    ) {
        public static RetryPolicy defaultPolicy() {
            return new RetryPolicy(3, 1000, 2.0);
        }
    }
}
