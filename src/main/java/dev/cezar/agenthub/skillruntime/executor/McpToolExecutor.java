package dev.cezar.agenthub.skillruntime.executor;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.cezar.agenthub.skillruntime.domain.Tool;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;

import java.time.Duration;
import java.util.Map;

/**
 * Executor para tools do tipo MCP (Model Context Protocol).
 * Faz chamadas para servidores MCP externos usando JSON-RPC 2.0.
 * 
 * Configuration esperado:
 * {
 *   "serverUrl": "http://mcp-server:3000",
 *   "toolName": "read_file",           // nome da tool no servidor MCP
 *   "headers": {                        // headers customizados (opcional)
 *     "Authorization": "Bearer token"
 *   },
 *   "timeout": 30000                    // timeout em ms (default: 30s)
 * }
 * 
 * Parameters esperado:
 * {
 *   "path": "/path/to/file",
 *   "encoding": "utf-8"
 * }
 * 
 * O executor:
 * 1. Monta uma requisição JSON-RPC 2.0 no formato:
 *    {
 *      "jsonrpc": "2.0",
 *      "id": "execution-{executionId}",
 *      "method": "tools/call",
 *      "params": {
 *        "name": "{toolName}",
 *        "arguments": {input}
 *      }
 *    }
 * 
 * 2. Envia para o servidor MCP via HTTP POST
 * 
 * 3. Retorna a resposta no formato:
 *    {
 *      "content": [...],     // conteúdo retornado pela tool MCP
 *      "isError": false
 *    }
 * 
 * Referência: https://spec.modelcontextprotocol.io/specification/2024-11-05/server/tools/
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class McpToolExecutor implements ToolExecutor {
    
    private final ObjectMapper objectMapper;
    private final WebClient.Builder webClientBuilder;
    
    private static final int DEFAULT_TIMEOUT_MS = 30000;
    private static final String JSON_RPC_VERSION = "2.0";
    private static final String MCP_TOOLS_CALL_METHOD = "tools/call";
    
    @Override
    public Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context) {
        return Mono.fromCallable(() -> parseConfiguration(tool.config()))
                .flatMap(config -> callMcpServer(config, input, context))
                .onErrorResume(error -> {
                    log.error("MCP tool call failed: {}", error.getMessage(), error);
                    return Mono.just(Map.of(
                            "success", false,
                            "error", error.getMessage(),
                            "isError", true
                    ));
                });
    }
    
    @Override
    public Mono<Void> validate(Tool tool, Map<String, Object> input) {
        return Mono.fromRunnable(() -> {
            McpConfig config = parseConfiguration(tool.config());
            
            if (config.serverUrl == null || config.serverUrl.isBlank()) {
                throw new IllegalArgumentException("MCP server URL is required");
            }
            
            if (config.toolName == null || config.toolName.isBlank()) {
                throw new IllegalArgumentException("MCP tool name is required");
            }
            
            if (!config.serverUrl.startsWith("http://") && !config.serverUrl.startsWith("https://")) {
                throw new IllegalArgumentException("MCP server URL must start with http:// or https://");
            }
            
            if (config.timeout != null && config.timeout <= 0) {
                throw new IllegalArgumentException("Timeout must be positive");
            }
        });
    }
    
    @Override
    public String getSupportedType() {
        return "MCP";
    }
    
    private McpConfig parseConfiguration(Map<String, Object> config) {
        try {
            return objectMapper.convertValue(config, McpConfig.class);
        } catch (Exception e) {
            throw new IllegalArgumentException("Invalid MCP configuration: " + e.getMessage(), e);
        }
    }
    
    private Mono<Map<String, Object>> callMcpServer(
            McpConfig config,
            Map<String, Object> input,
            ExecutionContext context) {
        
        // Constrói requisição JSON-RPC 2.0
        Map<String, Object> jsonRpcRequest = Map.of(
                "jsonrpc", JSON_RPC_VERSION,
                "id", "execution-" + (context.executionId() != null ? context.executionId() : "unknown"),
                "method", MCP_TOOLS_CALL_METHOD,
                "params", Map.of(
                        "name", config.toolName,
                        "arguments", input != null ? input : Map.of()
                )
        );
        
        log.debug("Calling MCP server: url={}, tool={}, executionId={}", 
                config.serverUrl, config.toolName, context.executionId());
        
        // Configura WebClient
        WebClient client = webClientBuilder
                .baseUrl(config.serverUrl)
                .defaultHeader(HttpHeaders.USER_AGENT, "AgentHub-SkillRuntime/1.0")
                .build();
        
        // Faz chamada HTTP POST
        WebClient.RequestBodySpec request = client.post()
                .contentType(MediaType.APPLICATION_JSON);
        
        // Adiciona headers customizados
        if (config.headers != null) {
            config.headers.forEach(request::header);
        }
        
        int timeout = config.timeout != null ? config.timeout : DEFAULT_TIMEOUT_MS;
        
        return request
                .bodyValue(jsonRpcRequest)
                .retrieve()
                .bodyToMono(Map.class)
                .timeout(Duration.ofMillis(timeout))
                .map(this::processMcpResponse)
                .onErrorMap(error -> new RuntimeException("MCP server call failed: " + error.getMessage(), error));
    }
    
    /**
     * Processa resposta JSON-RPC 2.0 do servidor MCP.
     * 
     * Formato esperado de sucesso:
     * {
     *   "jsonrpc": "2.0",
     *   "id": "...",
     *   "result": {
     *     "content": [...]
     *   }
     * }
     * 
     * Formato de erro:
     * {
     *   "jsonrpc": "2.0",
     *   "id": "...",
     *   "error": {
     *     "code": -32600,
     *     "message": "Error message"
     *   }
     * }
     */
    @SuppressWarnings("unchecked")
    private Map<String, Object> processMcpResponse(Map<?, ?> response) {
        // Verifica se é resposta de erro JSON-RPC
        if (response.containsKey("error")) {
            Map<String, Object> error = (Map<String, Object>) response.get("error");
            return Map.of(
                    "success", false,
                    "error", error.get("message"),
                    "errorCode", error.getOrDefault("code", -1),
                    "isError", true
            );
        }
        
        // Extrai resultado
        if (response.containsKey("result")) {
            Map<String, Object> result = (Map<String, Object>) response.get("result");
            
            return Map.of(
                    "success", true,
                    "content", result.getOrDefault("content", result),
                    "isError", false
            );
        }
        
        // Resposta inesperada
        return Map.of(
                "success", false,
                "error", "Invalid JSON-RPC response format",
                "rawResponse", response,
                "isError", true
        );
    }
    
    private static class McpConfig {
        public String serverUrl;
        public String toolName;
        public Map<String, String> headers;
        public Integer timeout;
    }
}
