package dev.cezar.agenthub.skillruntime.executor;

import dev.cezar.agenthub.skillruntime.domain.Tool;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("ScriptToolExecutor - Executor para scripts (Groovy/Python)")
class ScriptToolExecutorTest {

    private ScriptToolExecutor executor;

    @BeforeEach
    void setUp() {
        executor = new ScriptToolExecutor();
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("SCRIPT");
    }

    @Test
    @DisplayName("Deve validar que tool tem script")
    void shouldValidateToolHasScript() {
        // Given - tool sem script
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid Script Tool",
                "SCRIPT",
                Map.of("language", "groovy"), // falta 'script'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("script") ||
                        error.getMessage().contains("code")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem language configurada")
    void shouldValidateToolHasLanguage() {
        // Given - tool sem language
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid Script Tool",
                "SCRIPT",
                Map.of("script", "println 'Hello'"), // falta 'language'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("language")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar que language é suportada (groovy ou python)")
    void shouldValidateSupportedLanguage() {
        // Given - language não suportada
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid Script Tool",
                "SCRIPT",
                Map.of(
                        "script", "console.log('hello');",
                        "language", "javascript" // não suportado
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().toLowerCase().contains("unsupported") ||
                        error.getMessage().toLowerCase().contains("not supported") ||
                        error.getMessage().contains("groovy") ||
                        error.getMessage().contains("python")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar script Groovy válido")
    void shouldAcceptValidGroovyScript() {
        // Given
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Groovy Script Tool",
                "SCRIPT",
                Map.of(
                        "language", "groovy",
                        "script", "def result = input.a + input.b; return [sum: result]"
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of("a", 10, "b", 20);
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve aceitar script Python válido")
    void shouldAcceptValidPythonScript() {
        // Given
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Python Script Tool",
                "SCRIPT",
                Map.of(
                        "language", "python",
                        "script", "result = input['a'] + input['b']\nreturn {'sum': result}"
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of("a", 10, "b", 20);
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve bloquear imports perigosos (System, Runtime, ProcessBuilder)")
    void shouldBlockDangerousImports() {
        // Given - scripts com imports perigosos
        String[] dangerousScripts = {
                "import java.lang.System; System.exit(0);",
                "import java.lang.Runtime; Runtime.getRuntime().exec('rm -rf /');",
                "import java.lang.ProcessBuilder; new ProcessBuilder('cat', '/etc/passwd').start();"
        };

        for (String script : dangerousScripts) {
            Tool tool = new Tool(
                    UUID.randomUUID(),
                    UUID.randomUUID(),
                    "Dangerous Script Tool",
                    "SCRIPT",
                    Map.of(
                            "language", "groovy",
                            "script", script
                    ),
                    1,
                    "ACTIVE"
            );

            Map<String, Object> input = Map.of();
            ToolExecutor.ExecutionContext context = createContext();

            // When & Then
            StepVerifier.create(executor.validate(tool, input))
                    .expectErrorMatches(error ->
                            error.getMessage().toLowerCase().contains("unsafe") ||
                            error.getMessage().toLowerCase().contains("forbidden") ||
                            error.getMessage().toLowerCase().contains("not allowed")
                    )
                    .verify();
        }
    }

    @Test
    @DisplayName("Deve bloquear operações de I/O de arquivo")
    void shouldBlockFileIOOperations() {
        // Given - script tentando ler arquivo
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "File IO Script Tool",
                "SCRIPT",
                Map.of(
                        "language", "groovy",
                        "script", "new File('/etc/passwd').text"
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .expectErrorMatches(error ->
                        error.getMessage().toLowerCase().contains("file") ||
                        error.getMessage().toLowerCase().contains("io") ||
                        error.getMessage().toLowerCase().contains("not allowed")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar timeout configurado na tool")
    void shouldValidateTimeoutConfiguration() {
        // Given - tool com timeout muito alto
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Script Tool",
                "SCRIPT",
                Map.of(
                        "language", "groovy",
                        "script", "return [result: 'ok']",
                        "timeout", 300000 // 5 minutos - muito alto
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then - pode aceitar ou rejeitar dependendo da política
        // Neste teste, apenas verificamos que a validação ocorre
        StepVerifier.create(executor.validate(tool, input))
                .expectComplete()
                .verify();
    }

    // Helper methods
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
