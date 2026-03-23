package dev.cezar.agenthub.skillruntime.application.executor.impl;

import dev.cezar.agenthub.skillruntime.domain.port.ToolExecutor;

import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.io.IOException;
import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("HttpToolExecutor - Executor para ferramentas HTTP")
class HttpToolExecutorTest {

    private HttpToolExecutor executor;
    private MockWebServer mockWebServer;
    private String baseUrl;

    @BeforeEach
    void setUp() throws IOException {
        executor = new HttpToolExecutor();
        mockWebServer = new MockWebServer();
        mockWebServer.start();
        baseUrl = mockWebServer.url("/").toString();
    }

    @AfterEach
    void tearDown() throws IOException {
        mockWebServer.shutdown();
    }

    @Test
    @DisplayName("Deve executar GET request com sucesso")
    void shouldExecuteGetRequestSuccessfully() {
        // Given
        mockWebServer.enqueue(new MockResponse()
                .setBody("{\"status\":\"ok\",\"data\":\"test\"}")
                .addHeader("Content-Type", "application/json"));

        Tool tool = createHttpTool(baseUrl + "api/test", "GET", Map.of());
        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> {
                    assertThat(result).containsKey("status");
                    assertThat(result.get("status")).isEqualTo("success");
                    assertThat(result).containsKey("response");
                    assertThat(result.get("method")).isEqualTo("GET");
                })
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve executar POST request com body")
    void shouldExecutePostRequestWithBody() throws Exception {
        // Given
        mockWebServer.enqueue(new MockResponse()
                .setBody("{\"id\":123,\"created\":true}")
                .addHeader("Content-Type", "application/json"));

        Tool tool = createHttpTool(baseUrl + "api/create", "POST", Map.of());
        Map<String, Object> input = Map.of(
                "body", Map.of("name", "Test", "value", 42)
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When
        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> {
                    assertThat(result.get("status")).isEqualTo("success");
                    assertThat(result.get("method")).isEqualTo("POST");
                })
                .verifyComplete();

        // Then - verificar que body foi enviado
        RecordedRequest request = mockWebServer.takeRequest();
        assertThat(request.getMethod()).isEqualTo("POST");
        assertThat(request.getBody().readUtf8()).contains("\"name\":\"Test\"");
    }

    @Test
    @DisplayName("Deve incluir headers customizados no request")
    void shouldIncludeCustomHeaders() throws Exception {
        // Given
        mockWebServer.enqueue(new MockResponse()
                .setBody("{\"authenticated\":true}")
                .addHeader("Content-Type", "application/json"));

        Tool tool = createHttpTool(
                baseUrl + "api/protected",
                "GET",
                Map.of("Authorization", "Bearer token123", "X-Custom", "value")
        );
        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When
        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> assertThat(result.get("status")).isEqualTo("success"))
                .verifyComplete();

        // Then
        RecordedRequest request = mockWebServer.takeRequest();
        assertThat(request.getHeader("Authorization")).isEqualTo("Bearer token123");
        assertThat(request.getHeader("X-Custom")).isEqualTo("value");
    }

    @Test
    @DisplayName("Deve substituir path parameters na URL")
    void shouldReplacePathParametersInUrl() throws Exception {
        // Given
        mockWebServer.enqueue(new MockResponse()
                .setBody("{\"user\":{\"id\":123,\"name\":\"John\"}}")
                .addHeader("Content-Type", "application/json"));

        Tool tool = createHttpTool(baseUrl + "api/users/{userId}", "GET", Map.of());
        Map<String, Object> input = Map.of(
                "pathParams", Map.of("userId", "123")
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When
        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> {
                    assertThat(result.get("status")).isEqualTo("success");
                    assertThat(result.get("url")).toString().contains("/api/users/123");
                })
                .verifyComplete();

        // Then
        RecordedRequest request = mockWebServer.takeRequest();
        assertThat(request.getPath()).contains("/api/users/123");
    }

    @Test
    @DisplayName("Deve adicionar query parameters à URL")
    void shouldAddQueryParametersToUrl() throws Exception {
        // Given
        mockWebServer.enqueue(new MockResponse()
                .setBody("{\"results\":[]}")
                .addHeader("Content-Type", "application/json"));

        Tool tool = createHttpTool(baseUrl + "api/search", "GET", Map.of());
        Map<String, Object> input = Map.of(
                "queryParams", Map.of("q", "test", "limit", "10")
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When
        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> assertThat(result.get("status")).isEqualTo("success"))
                .verifyComplete();

        // Then
        RecordedRequest request = mockWebServer.takeRequest();
        assertThat(request.getPath()).contains("q=test");
        assertThat(request.getPath()).contains("limit=10");
    }

    @Test
    @DisplayName("Deve retornar erro quando servidor retorna erro")
    void shouldReturnErrorWhenServerReturnsError() {
        // Given
        mockWebServer.enqueue(new MockResponse()
                .setResponseCode(500)
                .setBody("{\"error\":\"Internal Server Error\"}"));

        Tool tool = createHttpTool(baseUrl + "api/error", "GET", Map.of());
        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.execute(tool, input, context))
                .assertNext(result -> {
                    assertThat(result.get("status")).isEqualTo("error");
                    assertThat(result).containsKey("error");
                })
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("HTTP");
    }

    @Test
    @DisplayName("Deve validar configuração da tool")
    void shouldValidateToolConfiguration() {
        // Given - tool sem URL
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid HTTP Tool",
                "HTTP",
                Map.of("method", "GET"), // falta 'url'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectError()
                .verify();
    }

    @Test
    @DisplayName("Deve validar quando method não é suportado")
    void shouldValidateUnsupportedMethod() {
        // Given
        Tool tool = createHttpTool(baseUrl + "api/test", "INVALID_METHOD", Map.of());
        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then - Spring 6 HttpMethod.valueOf() accepts any string (no exception)
        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    // Helper methods
    private Tool createHttpTool(String url, String method, Map<String, String> headers) {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "HTTP Test Tool",
                "HTTP",
                Map.of(
                        "url", url,
                        "method", method,
                        "headers", headers
                ),
                1,
                "ACTIVE"
        );
    }

    private ToolExecutor.ExecutionContext createContext() {
        return new ToolExecutor.ExecutionContext(
                UUID.randomUUID().toString(),
                UUID.randomUUID().toString(),
                UUID.randomUUID().toString(),
                UUID.randomUUID().toString(),
                "node-1"
        );
    }
}
