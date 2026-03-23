package dev.cezar.agenthub.skillruntime.domain.port;

import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import reactor.core.publisher.Mono;

import java.util.Map;

/**
 * Interface base para executores de tools.
 * <p>
 * Cada tipo de tool (HTTP, SQL, DOCUMENT_SEARCH, etc.) tem seu próprio executor
 * que implementa esta interface.
 * </p>
 *
 * @since 1.0.0
 */
public interface ToolExecutor {

    /**
     * Retorna o tipo de tool que este executor suporta.
     *
     * @return tipo (HTTP, SQL, DOCUMENT_SEARCH, MCP, SCRIPT)
     */
    String getSupportedType();

    /**
     * Executa a tool com os inputs fornecidos.
     *
     * @param tool   tool a ser executada
     * @param input  inputs para a execução
     * @param context contexto de execução (tenantId, userId, etc.)
     * @return {@link Mono} com o resultado da execução
     */
    Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context);

    /**
     * Valida se os inputs estão corretos para esta tool.
     *
     * @param tool  tool a ser validada
     * @param input inputs para validação
     * @return {@link Mono} que completa se válido, ou emite erro se inválido
     */
    default Mono<Void> validate(Tool tool, Map<String, Object> input) {
        return Mono.empty();
    }

    /**
     * Contexto de execução da tool.
     *
     * @param tenantId    ID do tenant
     * @param userId      ID do usuário
     * @param agentId     ID do agente
     * @param executionId ID da execução do agente
     * @param nodeId      ID do nó do pipeline
     */
    record ExecutionContext(
            String tenantId,
            String userId,
            String agentId,
            String executionId,
            String nodeId
    ) {}
}
