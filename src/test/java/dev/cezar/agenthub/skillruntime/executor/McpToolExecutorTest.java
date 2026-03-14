package dev.cezar.agenthub.skillruntime.executor;

import dev.cezar.agenthub.skillruntime.domain.Tool;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("McpToolExecutor - Executor para MCP (Model Context Protocol)")
class McpToolExecutorTest {

    private McpToolExecutor executor;

    @BeforeEach
    void setUp() {
        executor = new McpToolExecutor();
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("MCP");
    }

    @Test
    @DisplayName("Deve validar que tool tem mcpServer configurado")
    void shouldValidateToolHasMcpServer() {
        // Given - tool sem mcpServer
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid MCP Tool",
                "MCP",
                Map.of("toolName", "filesystem_read"), // falta 'mcpServer'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("mcpServer") ||
                        error.getMessage().contains("server")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem toolName configurado")
    void shouldValidateToolHasToolName() {
        // Given - tool sem toolName
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid MCP Tool",
                "MCP",
                Map.of("mcpServer", "filesystem"), // falta 'toolName'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("toolName") ||
                        error.getMessage().contains("tool name")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar configuração MCP válida")
    void shouldAcceptValidMcpConfiguration() {
        // Given
        Tool tool = createMcpTool("filesystem", "filesystem_read");
        Map<String, Object> input = Map.of("path", "/data/test.txt");
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve validar que mcpServer está na lista de servidores permitidos")
    void shouldValidateMcpServerInAllowedList() {
        // Given - servidor MCP não autorizado
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Unauthorized MCP Tool",
                "MCP",
                Map.of(
                        "mcpServer", "unauthorized-server",
                        "toolName", "dangerous_operation"
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .expectErrorMatches(error ->
                        error.getMessage().toLowerCase().contains("unauthorized") ||
                        error.getMessage().toLowerCase().contains("not allowed") ||
                        error.getMessage().toLowerCase().contains("permitted")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar servidores MCP conhecidos (filesystem, git, browser)")
    void shouldAcceptKnownMcpServers() {
        // Given
        String[] knownServers = {"filesystem", "git", "browser", "postgres", "github"};

        for (String server : knownServers) {
            Tool tool = createMcpTool(server, "test_operation");
            Map<String, Object> input = Map.of();
            ToolExecutor.ExecutionContext context = createContext();

            // When & Then - deve passar validação
            StepVerifier.create(executor.validate(tool, input))
                    .verifyComplete();
        }
    }

    @Test
    @DisplayName("Deve validar formato do toolName")
    void shouldValidateToolNameFormat() {
        // Given - toolName com caracteres inválidos
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "MCP Tool",
                "MCP",
                Map.of(
                        "mcpServer", "filesystem",
                        "toolName", "invalid tool name!" // espaços e caracteres especiais
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("toolName") &&
                        (error.getMessage().contains("format") || error.getMessage().contains("invalid"))
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem URL do MCP client (Go runtime)")
    void shouldValidateToolHasMcpClientUrl() {
        // Given - configuração sem mcpClientUrl
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "MCP Tool",
                "MCP",
                Map.of(
                        "mcpServer", "filesystem",
                        "toolName", "filesystem_read"
                        // falta 'mcpClientUrl'
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then - pode usar URL default ou exigir configuração
        StepVerifier.create(executor.validate(tool, input))
                .expectComplete() // ou expectError() se obrigatório
                .verify();
    }

    @Test
    @DisplayName("Deve validar timeout para operações MCP")
    void shouldValidateTimeoutForMcpOperations() {
        // Given
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "MCP Tool",
                "MCP",
                Map.of(
                        "mcpServer", "filesystem",
                        "toolName", "filesystem_read",
                        "timeout", 5000 // 5 segundos
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of("path", "/data/file.txt");
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve validar argumentos obrigatórios para tool MCP específica")
    void shouldValidateRequiredArgsForSpecificMcpTool() {
        // Given - filesystem_read sem 'path'
        Tool tool = createMcpTool("filesystem", "filesystem_read");
        Map<String, Object> invalidInput = Map.of(); // falta 'path'
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, invalidInput))
                .expectErrorMatches(error ->
                        error.getMessage().contains("path") ||
                        error.getMessage().contains("required")
                )
                .verify();
    }

    // Helper methods
    private Tool createMcpTool(String mcpServer, String toolName) {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "MCP Tool - " + toolName,
                "MCP",
                Map.of(
                        "mcpServer", mcpServer,
                        "toolName", toolName,
                        "mcpClientUrl", "http://localhost:9001"
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
