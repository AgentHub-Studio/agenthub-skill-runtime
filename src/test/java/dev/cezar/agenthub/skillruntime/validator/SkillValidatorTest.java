package dev.cezar.agenthub.skillruntime.validator;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.cezar.agenthub.skillruntime.domain.Skill;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.util.List;
import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("SkillValidator - Validação de inputs usando JSON Schema")
class SkillValidatorTest {

    private SkillValidator validator;
    private ObjectMapper objectMapper;

    @BeforeEach
    void setUp() {
        objectMapper = new ObjectMapper();
        validator = new SkillValidator(objectMapper);
    }

    @Test
    @DisplayName("Deve passar quando input válido")
    void shouldPassWithValidInput() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "required", List.of("name", "age"),
                        "properties", Map.of(
                                "name", Map.of("type", "string"),
                                "age", Map.of("type", "integer", "minimum", 0)
                        )
                )
        ));

        Map<String, Object> validInput = Map.of(
                "name", "John Doe",
                "age", 30
        );

        // When & Then
        StepVerifier.create(validator.validateInput(skill, validInput))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve falhar quando campo obrigatório ausente")
    void shouldFailWhenRequiredFieldMissing() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "required", List.of("email"),
                        "properties", Map.of(
                                "email", Map.of("type", "string", "format", "email")
                        )
                )
        ));

        Map<String, Object> invalidInput = Map.of(); // missing 'email'

        // When & Then
        StepVerifier.create(validator.validateInput(skill, invalidInput))
                .expectErrorMatches(error ->
                        error instanceof SkillValidator.SkillInputValidationException &&
                        error.getMessage().contains("email")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve falhar quando tipo incorreto")
    void shouldFailWhenTypeIncorrect() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "properties", Map.of(
                                "count", Map.of("type", "integer")
                        )
                )
        ));

        Map<String, Object> invalidInput = Map.of(
                "count", "not a number" // string em vez de integer
        );

        // When & Then
        StepVerifier.create(validator.validateInput(skill, invalidInput))
                .expectErrorMatches(error ->
                        error instanceof SkillValidator.SkillInputValidationException &&
                        error.getMessage().contains("integer")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar pattern regex")
    void shouldValidateRegexPattern() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "properties", Map.of(
                                "code", Map.of(
                                        "type", "string",
                                        "pattern", "^[A-Z]{3}-\\d{3}$" // formato: ABC-123
                                )
                        )
                )
        ));

        Map<String, Object> validInput = Map.of("code", "ABC-123");
        Map<String, Object> invalidInput = Map.of("code", "invalid"); // não match pattern

        // When & Then - valid
        StepVerifier.create(validator.validateInput(skill, validInput))
                .verifyComplete();

        // When & Then - invalid
        StepVerifier.create(validator.validateInput(skill, invalidInput))
                .expectError(SkillValidator.SkillInputValidationException.class)
                .verify();
    }

    @Test
    @DisplayName("Deve validar constraints de tamanho")
    void shouldValidateSizeConstraints() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "properties", Map.of(
                                "name", Map.of(
                                        "type", "string",
                                        "minLength", 3,
                                        "maxLength", 10
                                )
                        )
                )
        ));

        Map<String, Object> tooShort = Map.of("name", "AB"); // < 3 chars
        Map<String, Object> tooLong = Map.of("name", "ABCDEFGHIJK"); // > 10 chars

        // When & Then - too short
        StepVerifier.create(validator.validateInput(skill, tooShort))
                .expectError(SkillValidator.SkillInputValidationException.class)
                .verify();

        // When & Then - too long
        StepVerifier.create(validator.validateInput(skill, tooLong))
                .expectError(SkillValidator.SkillInputValidationException.class)
                .verify();
    }

    @Test
    @DisplayName("Deve validar enums")
    void shouldValidateEnums() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "properties", Map.of(
                                "status", Map.of(
                                        "type", "string",
                                        "enum", List.of("ACTIVE", "INACTIVE", "PENDING")
                                )
                        )
                )
        ));

        Map<String, Object> validInput = Map.of("status", "ACTIVE");
        Map<String, Object> invalidInput = Map.of("status", "INVALID_STATUS");

        // When & Then - valid
        StepVerifier.create(validator.validateInput(skill, validInput))
                .verifyComplete();

        // When & Then - invalid
        StepVerifier.create(validator.validateInput(skill, invalidInput))
                .expectError(SkillValidator.SkillInputValidationException.class)
                .verify();
    }

    @Test
    @DisplayName("Deve validar arrays")
    void shouldValidateArrays() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "properties", Map.of(
                                "tags", Map.of(
                                        "type", "array",
                                        "items", Map.of("type", "string"),
                                        "minItems", 1,
                                        "maxItems", 5
                                )
                        )
                )
        ));

        Map<String, Object> validInput = Map.of("tags", List.of("tag1", "tag2"));
        Map<String, Object> emptyArray = Map.of("tags", List.of()); // < minItems
        Map<String, Object> wrongType = Map.of("tags", List.of(1, 2, 3)); // numbers not strings

        // When & Then - valid
        StepVerifier.create(validator.validateInput(skill, validInput))
                .verifyComplete();

        // When & Then - empty
        StepVerifier.create(validator.validateInput(skill, emptyArray))
                .expectError(SkillValidator.SkillInputValidationException.class)
                .verify();

        // When & Then - wrong type
        StepVerifier.create(validator.validateInput(skill, wrongType))
                .expectError(SkillValidator.SkillInputValidationException.class)
                .verify();
    }

    @Test
    @DisplayName("Deve pular validação quando skill não tem schema")
    void shouldSkipValidationWhenNoSchema() {
        // Given
        Skill skillWithoutSchema = new Skill(
                UUID.randomUUID(),
                "test-skill",
                "Test Skill",
                "TEST",
                "Test skill without schema",
                "ACTIVE",
                Map.of() // sem inputSchema
        );

        Map<String, Object> anyInput = Map.of("anything", "goes");

        // When & Then - deve passar sem validar
        StepVerifier.create(validator.validateInput(skillWithoutSchema, anyInput))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve validar objetos aninhados")
    void shouldValidateNestedObjects() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "required", List.of("user"),
                        "properties", Map.of(
                                "user", Map.of(
                                        "type", "object",
                                        "required", List.of("name", "email"),
                                        "properties", Map.of(
                                                "name", Map.of("type", "string"),
                                                "email", Map.of("type", "string", "format", "email")
                                        )
                                )
                        )
                )
        ));

        Map<String, Object> validInput = Map.of(
                "user", Map.of(
                        "name", "John",
                        "email", "john@example.com"
                )
        );

        Map<String, Object> invalidInput = Map.of(
                "user", Map.of(
                        "name", "John"
                        // missing email
                )
        );

        // When & Then - valid
        StepVerifier.create(validator.validateInput(skill, validInput))
                .verifyComplete();

        // When & Then - invalid
        StepVerifier.create(validator.validateInput(skill, invalidInput))
                .expectError(SkillValidator.SkillInputValidationException.class)
                .verify();
    }

    @Test
    @DisplayName("Deve capturar múltiplos erros de validação")
    void shouldCaptureMultipleValidationErrors() {
        // Given
        Skill skill = createSkillWithSchema(Map.of(
                "inputSchema", Map.of(
                        "type", "object",
                        "required", List.of("name", "email", "age"),
                        "properties", Map.of(
                                "name", Map.of("type", "string"),
                                "email", Map.of("type", "string", "format", "email"),
                                "age", Map.of("type", "integer", "minimum", 0)
                        )
                )
        ));

        Map<String, Object> multipleErrorsInput = Map.of(
                "name", 123, // wrong type
                "email", "not-an-email", // wrong format
                "age", -5 // violates minimum
        );

        // When & Then
        StepVerifier.create(validator.validateInput(skill, multipleErrorsInput))
                .expectErrorMatches(error -> {
                    if (!(error instanceof SkillValidator.SkillInputValidationException)) {
                        return false;
                    }
                    SkillValidator.SkillInputValidationException validationError =
                            (SkillValidator.SkillInputValidationException) error;
                    
                    // Deve ter múltiplos erros capturados
                    assertThat(validationError.getValidationErrors()).isNotEmpty();
                    return true;
                })
                .verify();
    }

    // Helper method
    private Skill createSkillWithSchema(Map<String, Object> schema) {
        return new Skill(
                UUID.randomUUID(),
                "test-skill",
                "Test Skill",
                "TEST",
                "Test skill for validation",
                "ACTIVE",
                schema
        );
    }
}
