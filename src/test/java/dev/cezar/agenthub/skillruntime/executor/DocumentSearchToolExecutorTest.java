package dev.cezar.agenthub.skillruntime.executor;

import dev.cezar.agenthub.skillruntime.domain.Tool;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.r2dbc.core.DatabaseClient;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@ExtendWith(MockitoExtension.class)
@DisplayName("DocumentSearchToolExecutor - Executor para busca semântica de documentos")
class DocumentSearchToolExecutorTest {

    @Mock
    private DatabaseClient databaseClient;

    @Mock
    private WebClient.Builder webClientBuilder;

    private DocumentSearchToolExecutor executor;

    @BeforeEach
    void setUp() {
        executor = new DocumentSearchToolExecutor(databaseClient, webClientBuilder);
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("DOCUMENT_SEARCH");
    }

    @Test
    @DisplayName("Deve validar que input tem campo 'query'")
    void shouldValidateInputHasQueryField() {
        Tool tool = createValidTool();
        Map<String, Object> input = Map.of("knowledgeBaseId", UUID.randomUUID().toString());

        StepVerifier.create(executor.validate(tool, input))
                .expectErrorMatches(error -> error.getMessage().contains("query"))
                .verify();
    }

    @Test
    @DisplayName("Deve validar que input tem campo 'knowledgeBaseId'")
    void shouldValidateInputHasKnowledgeBaseId() {
        Tool tool = createValidTool();
        Map<String, Object> input = Map.of("query", "test search");

        StepVerifier.create(executor.validate(tool, input))
                .expectErrorMatches(error -> error.getMessage().contains("knowledgeBaseId"))
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem embeddingServiceUrl configurada")
    void shouldValidateToolHasEmbeddingServiceUrl() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Document Search Tool",
                "DOCUMENT_SEARCH",
                Map.of(), // falta embeddingServiceUrl
                1,
                "ACTIVE"
        );
        Map<String, Object> input = Map.of(
                "query", "test search",
                "knowledgeBaseId", UUID.randomUUID().toString()
        );

        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error -> error.getMessage().contains("embeddingServiceUrl"))
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar input válido com query e knowledgeBaseId")
    void shouldAcceptValidInput() {
        Tool tool = createValidTool();
        Map<String, Object> input = Map.of(
                "query", "What is AgentHub architecture?",
                "knowledgeBaseId", UUID.randomUUID().toString()
        );

        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve aceitar input válido com campos opcionais")
    void shouldAcceptValidInputWithOptionalFields() {
        Tool tool = createValidTool();
        Map<String, Object> input = Map.of(
                "query", "test search",
                "knowledgeBaseId", UUID.randomUUID().toString(),
                "limit", 5,
                "minScore", 0.7
        );

        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    // Helper methods
    private Tool createValidTool() {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Document Search Tool",
                "DOCUMENT_SEARCH",
                Map.of("embeddingServiceUrl", "http://embedding-service:8000/embed"),
                1,
                "ACTIVE"
        );
    }
}
