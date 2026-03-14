package dev.cezar.agenthub.skillruntime.executor;

import jakarta.annotation.PostConstruct;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * Registry para gerenciar executores de tools.
 * <p>
 * Mantém um mapa de tipo de tool → executor correspondente.
 * Executores são auto-registrados via dependency injection.
 * </p>
 *
 * @since 1.0.0
 */
@Slf4j
@Component
public class ToolExecutorRegistry {

    private final Map<String, ToolExecutor> executors = new HashMap<>();
    private final List<ToolExecutor> injectedExecutors;

    public ToolExecutorRegistry(List<ToolExecutor> injectedExecutors) {
        this.injectedExecutors = injectedExecutors;
    }

    /**
     * Registra todos os executores injetados após construção.
     */
    @PostConstruct
    public void init() {
        injectedExecutors.forEach(executor -> {
            String type = executor.getSupportedType();
            executors.put(type, executor);
            log.info("Registered ToolExecutor: {} → {}", type, executor.getClass().getSimpleName());
        });
        
        log.info("ToolExecutorRegistry initialized with {} executors", executors.size());
    }

    /**
     * Busca executor por tipo de tool.
     *
     * @param toolType tipo da tool (HTTP, SQL, DOCUMENT_SEARCH, etc.)
     * @return {@link Optional} com executor se encontrado
     */
    public Optional<ToolExecutor> getExecutor(String toolType) {
        return Optional.ofNullable(executors.get(toolType));
    }

    /**
     * Retorna todos os tipos de tool suportados.
     *
     * @return set de tipos suportados
     */
    public java.util.Set<String> getSupportedTypes() {
        return executors.keySet();
    }
}
