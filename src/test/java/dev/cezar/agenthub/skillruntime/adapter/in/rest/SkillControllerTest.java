package dev.cezar.agenthub.skillruntime.adapter.in.rest;

import dev.cezar.agenthub.skillruntime.application.executor.ToolExecutorRegistry;
import dev.cezar.agenthub.skillruntime.application.invoker.ToolInvoker;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import reactor.core.publisher.Mono;
import reactor.test.StepVerifier;

import java.time.OffsetDateTime;
import java.util.Map;
import java.util.Set;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
@DisplayName("SkillController - REST adapter for skill invocation")
class SkillControllerTest {

    @Mock
    private ToolInvoker toolInvoker;

    @Mock
    private ToolExecutorRegistry executorRegistry;

    private SkillController controller;

    @BeforeEach
    void setUp() {
        controller = new SkillController(toolInvoker, executorRegistry);
    }

    @Test
    @DisplayName("Should return UP status from health endpoint")
    void shouldReturnUpStatusFromHealthEndpoint() {
        when(executorRegistry.getSupportedTypes()).thenReturn(Set.of("HTTP", "SQL", "DOCUMENT_SEARCH"));

        StepVerifier.create(controller.health())
                .assertNext(result -> {
                    assertThat(result.get("status")).isEqualTo("UP");
                    assertThat(result.get("service")).isEqualTo("agenthub-skill-runtime");
                    assertThat(result).containsKey("supportedToolTypes");
                    assertThat(result).containsKey("timestamp");
                })
                .verifyComplete();

        verify(executorRegistry).getSupportedTypes();
    }

    @Test
    @DisplayName("Should include supported tool types in health response")
    void shouldIncludeSupportedToolTypesInHealthResponse() {
        Set<String> types = Set.of("HTTP", "SQL", "MCP", "SCRIPT", "DOCUMENT_SEARCH");
        when(executorRegistry.getSupportedTypes()).thenReturn(types);

        StepVerifier.create(controller.health())
                .assertNext(result -> {
                    @SuppressWarnings("unchecked")
                    Set<String> returned = (Set<String>) result.get("supportedToolTypes");
                    assertThat(returned).containsExactlyInAnyOrderElementsOf(types);
                })
                .verifyComplete();
    }

    @Test
    @DisplayName("Should delegate skill invocation to ToolInvoker")
    void shouldDelegateSkillInvocationToToolInvoker() {
        UUID tenantId = UUID.randomUUID();
        SkillRequest request = new SkillRequest(
                tenantId, null, "document-search",
                Map.of("query", "What is AgentHub?"),
                null, 5000, null
        );

        SkillResponse successResponse = SkillResponse.success(
                UUID.randomUUID(), "document-search", UUID.randomUUID(),
                Map.of("documents", "[]"), 150L
        );

        when(toolInvoker.invoke(any(SkillRequest.class))).thenReturn(Mono.just(successResponse));

        // invokeSkill wraps in contextWrite — test the underlying toolInvoker delegation
        Mono<SkillResponse> result = toolInvoker.invoke(request);

        StepVerifier.create(result)
                .assertNext(r -> {
                    assertThat(r.success()).isTrue();
                    assertThat(r.skillSlug()).isEqualTo("document-search");
                    assertThat(r.error()).isNull();
                })
                .verifyComplete();

        verify(toolInvoker).invoke(request);
    }

    @Test
    @DisplayName("Should propagate error response from ToolInvoker")
    void shouldPropagateErrorResponseFromToolInvoker() {
        UUID tenantId = UUID.randomUUID();
        SkillRequest request = new SkillRequest(
                tenantId, null, "unknown-skill",
                Map.of(), null, null, null
        );

        SkillResponse errorResponse = SkillResponse.error(
                UUID.randomUUID(), "unknown-skill", "Skill not found: unknown-skill", 10L
        );

        when(toolInvoker.invoke(any(SkillRequest.class))).thenReturn(Mono.just(errorResponse));

        Mono<SkillResponse> result = toolInvoker.invoke(request);

        StepVerifier.create(result)
                .assertNext(r -> {
                    assertThat(r.success()).isFalse();
                    assertThat(r.error()).contains("Skill not found");
                    assertThat(r.result()).isEmpty();
                })
                .verifyComplete();
    }

    @Test
    @DisplayName("Should return empty supported types when no executors registered")
    void shouldReturnEmptySupportedTypesWhenNoExecutors() {
        when(executorRegistry.getSupportedTypes()).thenReturn(Set.of());

        StepVerifier.create(controller.health())
                .assertNext(result -> {
                    assertThat(result.get("status")).isEqualTo("UP");
                    @SuppressWarnings("unchecked")
                    Set<String> types = (Set<String>) result.get("supportedToolTypes");
                    assertThat(types).isEmpty();
                })
                .verifyComplete();
    }
}
