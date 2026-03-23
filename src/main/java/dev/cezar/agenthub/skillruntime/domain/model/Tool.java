package dev.cezar.agenthub.skillruntime.domain.model;

import java.util.Map;
import java.util.UUID;

/**
 * Tool representa uma implementação concreta de uma skill.
 * <p>
 * Múltiplas tools podem implementar a mesma skill com diferentes provedores ou estratégias.
 * </p>
 *
 * @param id          ID da tool
 * @param skillId     ID da skill que esta tool implementa
 * @param name        Nome da tool
 * @param type        Tipo de executor (HTTP, SQL, DOCUMENT_SEARCH, MCP, SCRIPT)
 * @param config      Configuração específica da tool (URL, conexão, etc.)
 * @param priority    Prioridade de seleção (menor = maior prioridade)
 * @param status      Status (ACTIVE, INACTIVE)
 * @since 1.0.0
 */
public record Tool(
        UUID id,
        UUID skillId,
        String name,
        String type,
        Map<String, Object> config,
        Integer priority,
        String status
) {
    /**
     * Verifica se a tool está ativa.
     */
    public boolean isActive() {
        return "ACTIVE".equals(status);
    }
}
