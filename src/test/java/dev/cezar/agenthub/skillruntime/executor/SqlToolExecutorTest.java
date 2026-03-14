package dev.cezar.agenthub.skillruntime.executor;

import dev.cezar.agenthub.skillruntime.domain.Tool;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import reactor.test.StepVerifier;

import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("SqlToolExecutor - Executor para ferramentas SQL")
class SqlToolExecutorTest {

    private SqlToolExecutor executor;

    @BeforeEach
    void setUp() {
        executor = new SqlToolExecutor();
    }

    @Test
    @DisplayName("Deve retornar tipo suportado correto")
    void shouldReturnCorrectSupportedType() {
        assertThat(executor.getSupportedType()).isEqualTo("SQL");
    }

    @Test
    @DisplayName("Deve validar que tool tem query SQL")
    void shouldValidateToolHasSqlQuery() {
        // Given - tool sem query
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid SQL Tool",
                "SQL",
                Map.of("datasource", "postgresql"), // falta 'query'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("query") ||
                        error.getMessage().contains("SQL")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar que tool tem datasource configurado")
    void shouldValidateToolHasDatasource() {
        // Given - tool sem datasource
        Tool invalidTool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "Invalid SQL Tool",
                "SQL",
                Map.of("query", "SELECT * FROM users"), // falta 'datasource'
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(invalidTool, input))
                .expectErrorMatches(error ->
                        error.getMessage().contains("datasource") ||
                        error.getMessage().contains("database")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve validar query SQL contra SQL injection básico")
    void shouldValidateQueryAgainstBasicSqlInjection() {
        // Given - query com possível SQL injection
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "SQL Tool",
                "SQL",
                Map.of(
                        "datasource", "postgresql",
                        "query", "SELECT * FROM users WHERE id = ${id}; DROP TABLE users;"
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of("id", "1");
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .expectErrorMatches(error ->
                        error.getMessage().toLowerCase().contains("sql injection") ||
                        error.getMessage().toLowerCase().contains("unsafe") ||
                        error.getMessage().toLowerCase().contains("multiple statements")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve aceitar queries SELECT válidas")
    void shouldAcceptValidSelectQueries() {
        // Given
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "SQL Tool",
                "SQL",
                Map.of(
                        "datasource", "postgresql",
                        "query", "SELECT id, name, email FROM users WHERE active = true"
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of();
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then - deve passar validação
        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve aceitar queries com parâmetros parametrizados")
    void shouldAcceptParameterizedQueries() {
        // Given
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "SQL Tool",
                "SQL",
                Map.of(
                        "datasource", "postgresql",
                        "query", "SELECT * FROM orders WHERE user_id = :userId AND status = :status"
                ),
                1,
                "ACTIVE"
        );

        Map<String, Object> input = Map.of(
                "userId", 123,
                "status", "COMPLETED"
        );
        ToolExecutor.ExecutionContext context = createContext();

        // When & Then
        StepVerifier.create(executor.validate(tool, input))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve bloquear operações DML perigosas (DELETE sem WHERE)")
    void shouldBlockDangerousDmlOperations() {
        // Given - DELETE sem WHERE clause
        Tool tool = new Tool(
                UUID.randomUUID(),
                UUID.randomUUID(),
                "SQL Tool",
                "SQL",
                Map.of(
                        "datasource", "postgresql",
                        "query", "DELETE FROM users" // sem WHERE - perigoso!
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
                        error.getMessage().toLowerCase().contains("where clause required")
                )
                .verify();
    }

    @Test
    @DisplayName("Deve bloquear operações DDL (DROP, TRUNCATE, ALTER)")
    void shouldBlockDdlOperations() {
        // Given
        String[] dangerousQueries = {
                "DROP TABLE users",
                "TRUNCATE TABLE sessions",
                "ALTER TABLE users DROP COLUMN email"
        };

        for (String query : dangerousQueries) {
            Tool tool = new Tool(
                    UUID.randomUUID(),
                    UUID.randomUUID(),
                    "SQL Tool",
                    "SQL",
                    Map.of(
                            "datasource", "postgresql",
                            "query", query
                    ),
                    1,
                    "ACTIVE"
            );

            Map<String, Object> input = Map.of();
            ToolExecutor.ExecutionContext context = createContext();

            // When & Then
            StepVerifier.create(executor.validate(tool, input))
                    .expectErrorMatches(error ->
                            error.getMessage().toLowerCase().contains("ddl") ||
                            error.getMessage().toLowerCase().contains("not allowed") ||
                            error.getMessage().toLowerCase().contains("forbidden")
                    )
                    .verify();
        }
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
