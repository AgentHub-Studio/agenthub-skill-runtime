package dev.cezar.agenthub.skillruntime.application.executor;

import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import dev.cezar.agenthub.skillruntime.domain.port.ToolExecutor;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("ToolExecutorRegistry - Registration and lookup of tool executors")
class ToolExecutorRegistryTest {

    @Test
    @DisplayName("Should register all injected executors on init")
    void shouldRegisterAllInjectedExecutors() {
        ToolExecutor httpExecutor = stubExecutor("HTTP");
        ToolExecutor sqlExecutor = stubExecutor("SQL");

        ToolExecutorRegistry registry = new ToolExecutorRegistry(List.of(httpExecutor, sqlExecutor));
        registry.init();

        assertThat(registry.getSupportedTypes()).containsExactlyInAnyOrder("HTTP", "SQL");
    }

    @Test
    @DisplayName("Should return executor when type is registered")
    void shouldReturnExecutorForRegisteredType() {
        ToolExecutor executor = stubExecutor("DOCUMENT_SEARCH");
        ToolExecutorRegistry registry = new ToolExecutorRegistry(List.of(executor));
        registry.init();

        Optional<ToolExecutor> found = registry.getExecutor("DOCUMENT_SEARCH");

        assertThat(found).isPresent();
        assertThat(found.get().getSupportedType()).isEqualTo("DOCUMENT_SEARCH");
    }

    @Test
    @DisplayName("Should return empty Optional when type is not registered")
    void shouldReturnEmptyForUnregisteredType() {
        ToolExecutorRegistry registry = new ToolExecutorRegistry(List.of(stubExecutor("HTTP")));
        registry.init();

        Optional<ToolExecutor> found = registry.getExecutor("UNKNOWN_TYPE");

        assertThat(found).isEmpty();
    }

    @Test
    @DisplayName("Should return all supported types after registration")
    void shouldReturnAllSupportedTypes() {
        List<ToolExecutor> executors = List.of(
                stubExecutor("HTTP"),
                stubExecutor("SQL"),
                stubExecutor("MCP"),
                stubExecutor("SCRIPT")
        );

        ToolExecutorRegistry registry = new ToolExecutorRegistry(executors);
        registry.init();

        assertThat(registry.getSupportedTypes()).hasSize(4);
        assertThat(registry.getSupportedTypes()).contains("HTTP", "SQL", "MCP", "SCRIPT");
    }

    @Test
    @DisplayName("Should return empty set when no executors registered")
    void shouldReturnEmptySetWhenNoExecutors() {
        ToolExecutorRegistry registry = new ToolExecutorRegistry(List.of());
        registry.init();

        assertThat(registry.getSupportedTypes()).isEmpty();
        assertThat(registry.getExecutor("HTTP")).isEmpty();
    }

    @Test
    @DisplayName("Should overwrite executor when same type registered twice")
    void shouldOverwriteExecutorForDuplicateType() {
        ToolExecutor first = stubExecutor("HTTP");
        ToolExecutor second = stubExecutor("HTTP");

        ToolExecutorRegistry registry = new ToolExecutorRegistry(List.of(first, second));
        registry.init();

        assertThat(registry.getSupportedTypes()).hasSize(1);
        assertThat(registry.getExecutor("HTTP")).isPresent();
    }

    // Helper to create a minimal ToolExecutor stub

    private ToolExecutor stubExecutor(String type) {
        return new ToolExecutor() {
            @Override
            public String getSupportedType() {
                return type;
            }

            @Override
            public Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context) {
                return Mono.just(Map.of("type", type));
            }
        };
    }
}
