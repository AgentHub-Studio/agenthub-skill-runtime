package dev.cezar.agenthub.skillruntime.application.executor.impl;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("ScriptToolExecutor - Executor para scripts (Groovy)")
class ScriptToolExecutorTest {

    private ScriptToolExecutor executor;

    @BeforeEach
    void setUp() {
        // Use real ObjectMapper so convertValue() works with the private inner ScriptConfig class
        executor = new ScriptToolExecutor(new ObjectMapper());
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("SCRIPT");
    }

    @Test
    @DisplayName("Deve validar que tool tem script configurado")
    void shouldValidateToolHasScript() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid Script Tool",
                "SCRIPT",
                Map.of("language", "GROOVY"), // falta script
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(invalidTool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("script") || error.getMessage().contains("Script"))
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem language configurado")
    void shouldValidateToolHasLanguage() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid Script Tool",
                "SCRIPT",
                Map.of("script", "return ['result': 42]"), // falta language
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(invalidTool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("language") || error.getMessage().contains("Language"))
                .verify();
    }

    @Test
    @DisplayName("Deve validar que linguagem suportada é GROOVY")
    void shouldValidateSupportedLanguageIsGroovy() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Python Script Tool",
                "SCRIPT",
                Map.of(
                        "script", "print('hello')",
                        "language", "PYTHON" // não suportado
                ),
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(invalidTool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("GROOVY") || error.getMessage().contains("supported"))
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar configuração GROOVY válida")
    void shouldAcceptValidGroovyScript() {
        Tool tool = createScriptTool("return ['result': input.value * 2]", "GROOVY");

        StepVerifier.create(executor.validate(tool, Map.of()))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve aceitar linguagem em minúsculo (case insensitive)")
    void shouldAcceptGroovyLanguageCaseInsensitive() {
        Tool tool = createScriptTool("return ['result': 'ok']", "groovy");

        StepVerifier.create(executor.validate(tool, Map.of()))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve rejeitar timeout negativo ou zero")
    void shouldRejectNonPositiveTimeout() {
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Script Tool",
                "SCRIPT",
                Map.of(
                        "script", "return [:]",
                        "language", "GROOVY",
                        "timeout", -1
                ),
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(tool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("timeout") || error.getMessage().contains("Timeout"))
                .verify();
    }

    // Helper methods
    private Tool createScriptTool(String script, String language) {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Script Tool",
                "SCRIPT",
                Map.of(
                        "script", script,
                        "language", language
                ),
                1,
                "ACTIVE"
        );
    }
}
