package dev.cezar.agenthub.skillruntime.adapter.in.rest;

import io.swagger.v3.oas.annotations.media.Schema;
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
@Schema(description = "Request to invoke a skill with validation, retry, and timeout support")
public record SkillRequest(
        @NotNull
        @Schema(
            description = "Tenant ID for multi-tenancy isolation",
            example = "123e4567-e89b-12d3-a456-426614174000",
            required = true
        )
        UUID tenantId,

        @Schema(
            description = "Skill ID (optional, use skillSlug if not provided)",
            example = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
        )
        UUID skillId,

        @NotBlank
        @Schema(
            description = "Skill slug identifier (e.g., 'document-search', 'sql-query', 'http-post')",
            example = "document-search",
            required = true
        )
        String skillSlug,

        @NotNull
        @Schema(
            description = "Input parameters for the skill (validated against skill's JSON Schema)",
            example = "{\"query\": \"What is AgentHub?\", \"limit\": 5}",
            required = true
        )
        Map<String, Object> input,

        @Schema(description = "Execution context with agent, user, and node information")
        ExecutionContext executionContext,

        @Schema(
            description = "Timeout in milliseconds (default: 30000)",
            example = "5000"
        )
        Integer timeout,

        @Schema(description = "Retry policy configuration (default: 3 attempts, 1s backoff, 2.0 multiplier)")
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
    @Schema(description = "Execution context with agent, user, and workflow information")
    public record ExecutionContext(
            @Schema(description = "Agent ID executing the skill", example = "agent-001")
            UUID agentId,

            @Schema(description = "User ID who initiated the execution", example = "user-001")
            UUID userId,

            @Schema(description = "Execution ID of the agent workflow", example = "exec-001")
            UUID executionId,

            @Schema(description = "Node ID in the execution pipeline", example = "node-search-1")
            String nodeId
    ) {}

    /**
     * Política de retry para a execução.
     *
     * @param maxAttempts      Número máximo de tentativas (padrão: 3)
     * @param backoffMs        Backoff inicial em ms (padrão: 1000)
     * @param backoffMultiplier Multiplicador de backoff (padrão: 2.0)
     */
    @Schema(description = "Retry policy with exponential backoff")
    public record RetryPolicy(
            @Schema(
                description = "Maximum number of retry attempts (not counting initial attempt)",
                example = "3",
                minimum = "0",
                maximum = "10"
            )
            int maxAttempts,

            @Schema(
                description = "Initial backoff delay in milliseconds",
                example = "1000",
                minimum = "0"
            )
            long backoffMs,

            @Schema(
                description = "Backoff multiplier for exponential backoff (e.g., 2.0 = double each retry)",
                example = "2.0",
                minimum = "1.0"
            )
            double backoffMultiplier
    ) {
        public static RetryPolicy defaultPolicy() {
            return new RetryPolicy(3, 1000, 2.0);
        }
    }
}
