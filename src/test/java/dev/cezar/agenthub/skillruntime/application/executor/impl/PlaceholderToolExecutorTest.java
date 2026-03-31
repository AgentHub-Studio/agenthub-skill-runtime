package dev.cezar.agenthub.skillruntime.application.executor.impl;

import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import dev.cezar.agenthub.skillruntime.domain.port.ToolExecutor;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("PlaceholderToolExecutor - Stub executor for testing")
class PlaceholderToolExecutorTest {

    private PlaceholderToolExecutor executor;

    @BeforeEach
    void setUp() {
        executor = new PlaceholderToolExecutor();
    }

    @Test
    @DisplayName("Should return PLACEHOLDER as supported type")
    void shouldReturnPlaceholderSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("PLACEHOLDER");
    }

    @Test
    @DisplayName("Should return success result with tool name and input")
    void shouldReturnSuccessResultWithToolNameAndInput() {
        Tool tool = createTool("my-tool");
        Map<String, Object> input = Map.of("query", "test", "limit", 5);
        ToolExecutor.ExecutionContext context = createContext("tenant-1", "user-1", "agent-1");

        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> {
                    assertThat(result.get("status")).isEqualTo("success");
                    assertThat(result.get("tool")).isEqualTo("my-tool");
                    assertThat(result.get("message")).isEqualTo("Placeholder execution completed");
                    assertThat(result.get("input")).isEqualTo(input);
                    assertThat(result).containsKey("executedAt");
                })
                .verifyComplete();
    }

    @Test
    @DisplayName("Should include tenant and user context in result")
    void shouldIncludeTenantAndUserContextInResult() {
        Tool tool = createTool("context-tool");
        Map<String, Object> input = Map.of();
        String tenantId = "tenant-abc";
        String userId = "user-xyz";
        String agentId = "agent-001";
        ToolExecutor.ExecutionContext context = createContext(tenantId, userId, agentId);

        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> {
                    @SuppressWarnings("unchecked")
                    Map<String, Object> ctx = (Map<String, Object>) result.get("context");
                    assertThat(ctx.get("tenantId")).isEqualTo(tenantId);
                    assertThat(ctx.get("userId")).isEqualTo(userId);
                    assertThat(ctx.get("agentId")).isEqualTo(agentId);
                })
                .verifyComplete();
    }

    @Test
    @DisplayName("Should handle null userId and agentId gracefully")
    void shouldHandleNullUserIdAndAgentIdGracefully() {
        Tool tool = createTool("minimal-tool");
        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = new ToolExecutor.ExecutionContext(
                "tenant-1", null, null, UUID.randomUUID().toString(), "node-1"
        );

        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> {
                    @SuppressWarnings("unchecked")
                    Map<String, Object> ctx = (Map<String, Object>) result.get("context");
                    assertThat(ctx.get("userId")).isEqualTo("N/A");
                    assertThat(ctx.get("agentId")).isEqualTo("N/A");
                })
                .verifyComplete();
    }

    @Test
    @DisplayName("Should complete validation without error for any tool")
    void shouldCompleteValidationWithoutErrorForAnyTool() {
        Tool tool = createTool("validated-tool");
        Map<String, Object> input = Map.of("key", "value");

        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Should complete validation even with empty input")
    void shouldCompleteValidationWithEmptyInput() {
        Tool tool = createTool("empty-input-tool");

        StepVerifier.create(executor.validate(tool, Map.of()))
                .verifyComplete();
    }

    // Helpers

    private Tool createTool(String name) {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                name,
                "PLACEHOLDER",
                Map.of(),
                1,
                "ACTIVE"
        );
    }

    private ToolExecutor.ExecutionContext createContext(String tenantId, String userId, String agentId) {
        return new ToolExecutor.ExecutionContext(
                tenantId,
                userId,
                agentId,
                UUID.randomUUID().toString(),
                "node-1"
        );
    }
}
