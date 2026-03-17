package dev.cezar.agenthub.skillruntime.executor;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.cezar.agenthub.skillruntime.domain.Tool;
import io.r2dbc.spi.ConnectionFactory;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@ExtendWith(MockitoExtension.class)
@DisplayName("SqlToolExecutor - Executor para ferramentas SQL")
class SqlToolExecutorTest {

    @Mock
    private ConnectionFactory connectionFactory;

    private SqlToolExecutor executor;

    @BeforeEach
    void setUp() {
        // Use real ObjectMapper so convertValue() works with the private inner SqlConfig class
        executor = new SqlToolExecutor(connectionFactory, new ObjectMapper());
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("SQL");
    }

    @Test
    @DisplayName("Deve validar que tool tem query configurada")
    void shouldValidateToolHasQuery() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid SQL Tool",
                "SQL",
                Map.of("type", "SELECT"), // falta query
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(invalidTool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("query") || error.getMessage().contains("SQL"))
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem type configurado")
    void shouldValidateToolHasType() {
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid SQL Tool",
                "SQL",
                Map.of("query", "SELECT * FROM users"), // falta type
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(invalidTool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("type") || error.getMessage().contains("Type"))
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar configuração SQL válida")
    void shouldAcceptValidSqlConfiguration() {
        Tool tool = createSqlTool("SELECT * FROM users WHERE id = :id", "SELECT");

        StepVerifier.create(executor.validate(tool, Map.of()))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve rejeitar maxRows negativo ou zero")
    void shouldRejectNonPositiveMaxRows() {
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "SQL Tool",
                "SQL",
                Map.of(
                        "query", "SELECT * FROM users",
                        "type", "SELECT",
                        "maxRows", 0
                ),
                1,
                "ACTIVE"
        );

        StepVerifier.create(executor.validate(tool, Map.of()))
                .expectErrorMatches(error -> error.getMessage().contains("maxRows"))
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar query INSERT válida")
    void shouldAcceptValidInsertQuery() {
        Tool tool = createSqlTool("INSERT INTO audit_log (event) VALUES (:event)", "INSERT");

        StepVerifier.create(executor.validate(tool, Map.of()))
                .verifyComplete();
    }

    // Helper methods
    private Tool createSqlTool(String query, String type) {
        return new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "SQL Tool",
                "SQL",
                Map.of(
                        "query", query,
                        "type", type
                ),
                1,
                "ACTIVE"
        );
    }
}
