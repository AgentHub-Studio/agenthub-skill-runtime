package dev.cezar.agenthub.skillruntime.executor;

import dev.cezar.agenthub.skillruntime.domain.Tool;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("DocumentSearchToolExecutor - Executor para busca semântica de documentos")
class DocumentSearchToolExecutorTest {

    private DocumentSearchToolExecutor executor;

    @BeforeEach
    void setUp() {
        executor = new DocumentSearchToolExecutor();
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("DOCUMENT_SEARCH");
    }

    @Test
    @DisplayName("Deve validar que input tem campo 'query'")
    void shouldValidateInputHasQueryField() {
        // Given
        Tool tool = createDocumentSearchTool();
        Map<String, Object> invalidInput = Map.of("limit", 10); // falta 'query'
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, invalidInput))
                .expectErrorMatches(error ->
                        error.getMessage().contains("query") ||
                        error.getMessage().contains("required")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar que query não está vazia")
    void shouldValidateQueryNotEmpty() {
        // Given
        Tool tool = createDocumentSearchTool();
        Map<String, Object> invalidInput = Map.of("query", ""); // query vazia
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, invalidInput))
                .expectErrorMatches(error ->
                        error.getMessage().contains("empty") ||
                        error.getMessage().contains("blank")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar limite dentro de bounds razoáveis")
    void shouldValidateLimitWithinReasonableBounds() {
        // Given
        Tool tool = createDocumentSearchTool();
        
        // Limite muito alto
        Map<String, Object> tooHighLimit = Map.of(
                "query", "test query",
                "limit", 1000
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, tooHighLimit))
                .expectErrorMatches(error ->
                        error.getMessage().contains("limit") &&
                        (error.getMessage().contains("maximum") || error.getMessage().contains("too high"))
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar threshold entre 0 e 1")
    void shouldValidateThresholdBetweenZeroAndOne() {
        // Given
        Tool tool = createDocumentSearchTool();
        
        // Threshold inválido (> 1)
        Map<String, Object> invalidThreshold = Map.of(
                "query", "test query",
                "threshold", 1.5
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, invalidThreshold))
                .expectErrorMatches(error ->
                        error.getMessage().contains("threshold") &&
                        (error.getMessage().contains("0") && error.getMessage().contains("1"))
                )
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar input válido com todos os campos")
    void shouldAcceptValidInputWithAllFields() {
        // Given
        Tool tool = createDocumentSearchTool();
        Map<String, Object> validInput = Map.of(
                "query", "What is AgentHub architecture?",
                "limit", 5,
                "threshold", 0.7
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then - deve passar validação
        StepVerifier.create(executor.validate(tool, validInput))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve aceitar input válido apenas com query (campos opcionais)")
    void shouldAcceptValidInputWithOnlyQuery() {
        // Given
        Tool tool = createDocumentSearchTool();
        Map<String, Object> validInput = Map.of("query", "test search");
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, validInput))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve validar que tool tem collection configurada")
    void shouldValidateToolHasCollection() {
        // Given - tool sem collection
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid Document Search Tool",
                "DOCUMENT_SEARCH",
                Map.of("embeddingModel", "text-embedding-ada-002"), // falta 'collection'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of("query", "test");
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("collection") ||
                        error.getMessage().contains("namespace")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem embeddingModel configurado")
    void shouldValidateToolHasEmbeddingModel() {
        // Given - tool sem embeddingModel
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid Document Search Tool",
                "DOCUMENT_SEARCH",
                Map.of("collection", "documents"), // falta 'embeddingModel'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of("query", "test");
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("embedding") ||
                        error.getMessage().contains("model")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar tipo do campo query como String")
    void shouldValidateQueryFieldTypeAsString() {
        // Given
        Tool tool = createDocumentSearchTool();
        Map<String, Object> invalidInput = Map.of("query", 123); // número em vez de string
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, invalidInput))
                .expectErrorMatches(error ->
                        error.getMessage().contains("query") &&
                        (error.getMessage().contains("string") || error.getMessage().contains("type"))
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar tipo do campo limit como Integer")
    void shouldValidateLimitFieldTypeAsInteger() {
        // Given
        Tool tool = createDocumentSearchTool();
        Map<String, Object> invalidInput = Map.of(
                "query", "test",
                "limit", "not a number"
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, invalidInput))
                .expectErrorMatches(error ->
                        error.getMessage().contains("limit") &&
                        (error.getMessage().contains("integer") || error.getMessage().contains("number"))
                )
                .verify();
    }

    // Helper methods
    private Tool createDocumentSearchTool() {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Document Search Tool",
                "DOCUMENT_SEARCH",
                Map.of(
                        "collection", "knowledge_base",
                        "embeddingModel", "text-embedding-ada-002",
                        "vectorDimension", 1536
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
