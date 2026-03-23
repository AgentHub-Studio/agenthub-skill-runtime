package dev.cezar.agenthub.skillruntime.application.executor.impl;

import dev.cezar.agenthub.skillruntime.domain.port.ToolExecutor;

import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.time.OffsetDateTime;
import java.util.Map;

/**
 * Executor placeholder para testes.
 * <p>
 * Retorna resultado fictício sem executar nada de fato.
 * Usado até os executores reais serem implementados.
 * </p>
 *
 * @since 1.0.0
 */
@Slf4j
@Component
public class PlaceholderToolExecutor implements ToolExecutor {

    @Override
    public String getSupportedType() {
        return "PLACEHOLDER";
    }

    @Override
    public Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context) {
        log.info("Executing placeholder tool: toolId={}, input={}", tool.id(), input);

        // Simula processamento
        return Mono.just(Map.of(
                "status", "success",
                "message", "Placeholder execution completed",
                "tool", tool.name(),
                "input", input,
                "context", Map.of(
                        "tenantId", context.tenantId(),
                        "userId", context.userId() != null ? context.userId() : "N/A",
                        "agentId", context.agentId() != null ? context.agentId() : "N/A"
                ),
                "executedAt", OffsetDateTime.now().toString()
        ));
    }

    @Override
    public Mono<Void> validate(Tool tool, Map<String, Object> input) {
        // Placeholder sempre considera válido
        return Mono.empty();
    }
}
