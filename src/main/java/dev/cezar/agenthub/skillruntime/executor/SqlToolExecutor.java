package dev.cezar.agenthub.skillruntime.executor;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.cezar.agenthub.skillruntime.domain.Tool;
import io.r2dbc.spi.ConnectionFactory;
import io.r2dbc.spi.Row;
import io.r2dbc.spi.RowMetadata;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.util.*;

/**
 * Executor para tools do tipo SQL.
 * Executa queries SQL contra o banco de dados usando R2DBC.
 * 
 * Configuration esperado:
 * {
 *   "query": "SELECT * FROM users WHERE tenant_id = :tenantId LIMIT :limit",
 *   "type": "SELECT", // SELECT, INSERT, UPDATE, DELETE
 *   "maxRows": 1000,  // limite de linhas retornadas (default: 1000)
 *   "timeout": 30000  // timeout em ms (default: 30s)
 * }
 * 
 * Parameters esperado:
 * {
 *   "tenantId": "uuid",
 *   "limit": 10
 * }
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class SqlToolExecutor implements ToolExecutor {
    
    private final ConnectionFactory connectionFactory;
    private final ObjectMapper objectMapper;
    
    private static final int DEFAULT_MAX_ROWS = 1000;
    private static final int DEFAULT_TIMEOUT_MS = 30000;
    
    @Override
    public Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context) {
        return Mono.fromCallable(() -> parseConfiguration(tool.config()))
                .flatMap(config -> executeQuery(config, input, context))
                .map(result -> Map.of(
                        "success", true,
                        "data", result,
                        "rowCount", result instanceof List ? ((List<?>) result).size() : 0
                ))
                .onErrorResume(error -> {
                    log.error("SQL execution failed: {}", error.getMessage(), error);
                    return Mono.just(Map.of(
                            "success", false,
                            "error", error.getMessage()
                    ));
                });
    }
    
    @Override
    public Mono<Void> validate(Tool tool, Map<String, Object> input) {
        return Mono.fromRunnable(() -> {
            SqlConfig config = parseConfiguration(tool.config());
            
            if (config.query == null || config.query.isBlank()) {
                throw new IllegalArgumentException("SQL query is required");
            }
            
            if (config.type == null) {
                throw new IllegalArgumentException("Query type is required");
            }
            
            if (config.maxRows != null && config.maxRows <= 0) {
                throw new IllegalArgumentException("maxRows must be positive");
            }
            
            if (config.timeout != null && config.timeout <= 0) {
                throw new IllegalArgumentException("timeout must be positive");
            }
        });
    }
    
    @Override
    public String getSupportedType() {
        return "SQL";
    }
    
    private SqlConfig parseConfiguration(Map<String, Object> config) {
        try {
            return objectMapper.convertValue(config, SqlConfig.class);
        } catch (Exception e) {
            throw new IllegalArgumentException("Invalid SQL configuration: " + e.getMessage(), e);
        }
    }
    
    private Mono<Object> executeQuery(SqlConfig config, Map<String, Object> input, ExecutionContext context) {
        String query = config.query;
        int maxRows = config.maxRows != null ? config.maxRows : DEFAULT_MAX_ROWS;
        
        // Merge context into parameters for binding
        Map<String, Object> parameters = new HashMap<>();
        if (input != null) {
            parameters.putAll(input);
        }
        // Add context parameters for tenant isolation
        parameters.put("tenantId", context.tenantId());
        parameters.put("userId", context.userId());
        
        log.debug("Executing SQL query: {} with parameters: {}", query, parameters);
        
        return Mono.usingWhen(
                connectionFactory.create(),
                connection -> {
                    var statement = connection.createStatement(query);
                    
                    // Bind parameters
                    for (Map.Entry<String, Object> entry : parameters.entrySet()) {
                        statement.bind(entry.getKey(), entry.getValue());
                    }
                    
                    // Execute and process results
                    if ("SELECT".equalsIgnoreCase(config.type)) {
                        return Flux.from(statement.execute())
                                .flatMap(result -> result.map(this::mapRow))
                                .take(maxRows)
                                .collectList();
                    } else {
                        // INSERT, UPDATE, DELETE - return affected rows count
                        return Flux.from(statement.execute())
                                .flatMap(result -> Mono.from(result.getRowsUpdated()))
                                .reduce(0L, Long::sum)
                                .map(count -> Map.of("affectedRows", count));
                    }
                },
                connection -> Mono.from(connection.close())
        );
    }
    
    private Map<String, Object> mapRow(Row row, RowMetadata metadata) {
        Map<String, Object> result = new HashMap<>();
        
        metadata.getColumnMetadatas().forEach(columnMetadata -> {
            String columnName = columnMetadata.getName();
            try {
                Object value = row.get(columnName);
                result.put(columnName, value);
            } catch (Exception e) {
                log.warn("Failed to read column {}: {}", columnName, e.getMessage());
                result.put(columnName, null);
            }
        });
        
        return result;
    }
    
    private static class SqlConfig {
        public String query;
        public String type;
        public Integer maxRows;
        public Integer timeout;
    }
}
