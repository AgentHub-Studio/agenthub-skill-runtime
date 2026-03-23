package dev.cezar.agenthub.skillruntime.application.executor.impl;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@ExtendWith(MockitoExtension.class)
@DisplayName("McpToolExecutor - Executor para MCP (Model Context Protocol)")
class McpToolExecutorTest {

    @Mock
    private WebClient.Builder webClientBuilder;

    private McpToolExecutor executor;

    @BeforeEach
    void setUp() {
        // Use real ObjectMapper so convertValue() works with the private inner McpConfig class
        executor = new McpToolExecutor(new ObjectMapper(), webClientBuilder);
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("MCP");
    }

    @Test
    @DisplayName("Deve validar que tool tem serverUrl configurada")
    void shouldValidateToolHasServerUrl() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid MCP Tool",
                "MCP",
                Map.of("toolName", "filesystem_read"), // falta serverUrl
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(invalidTool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("server") || error.getMessage().contains("URL"))
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem toolName configurado")
    void shouldValidateToolHasToolName() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid MCP Tool",
                "MCP",
                Map.of("serverUrl", "http://mcp-server:3000"), // falta toolName
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(invalidTool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("tool name") || error.getMessage().contains("toolName"))
                .verify();
    }

    @Test
    @DisplayName("Deve validar que serverUrl começa com http:// ou https://")
    void shouldValidateServerUrlFormat() {
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "MCP Tool",
                "MCP",
                Map.of(
                        "serverUrl", "ftp://invalid-url",
                        "toolName", "filesystem_read"
                ),
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(tool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("http"))
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar configuração MCP válida com http://")
    void shouldAcceptValidMcpConfigurationHttp() {
        Tool tool = createMcpTool("http://mcp-server:3000", "filesystem_read");

        StepVerifier.create(executor.validate(tool, Map.of()))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve aceitar configuração MCP válida com https://")
    void shouldAcceptValidMcpConfigurationHttps() {
        Tool tool = createMcpTool("https://mcp-server.example.com", "git_commit");

        StepVerifier.create(executor.validate(tool, Map.of()))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve rejeitar timeout negativo ou zero")
    void shouldRejectNonPositiveTimeout() {
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "MCP Tool",
                "MCP",
                Map.of(
                        "serverUrl", "http://mcp-server:3000",
                        "toolName", "filesystem_read",
                        "timeout", 0
                ),
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(tool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("Timeout") || error.getMessage().contains("timeout"))
                .verify();
    }

    // Helper methods
    private Tool createMcpTool(String serverUrl, String toolName) {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "MCP Tool - " + toolName,
                "MCP",
                Map.of(
                        "serverUrl", serverUrl,
                        "toolName", toolName
                ),
                1,
                "ACTIVE"
        );
    }
}
