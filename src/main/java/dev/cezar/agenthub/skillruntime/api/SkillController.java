package dev.cezar.agenthub.skillruntime.api;

import dev.cezar.agenthub.skillruntime.executor.ToolExecutorRegistry;
import dev.cezar.agenthub.skillruntime.multitenant.TenantContext;
import dev.cezar.agenthub.skillruntime.multitenant.TenantContextHolder;
import dev.cezar.agenthub.skillruntime.service.ToolInvoker;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.ExampleObject;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.parameters.RequestBody;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.responses.ApiResponses;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.validation.Valid;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.time.OffsetDateTime;
import java.util.Map;

/**
 * Controller REST para invocar skills.
 * <p>
 * Skills são abstrações de capacidades (ex: "document-search", "send-email") que são
 * resolvidas para tools concretas (ex: pgvector, SendGrid). Este controller permite
 * invocar skills com validação JSON Schema, retry automático e timeout configurável.
 * </p>
 *
 * @since 1.0.0
 */
@Slf4j
@RestController
@RequestMapping("/api/v1/skills")
@AllArgsConstructor
@Tag(
    name = "Skill Runtime", 
    description = "Skill invocation and execution engine. " +
                  "Skills are abstract capabilities (e.g., 'document-search', 'sql-query') " +
                  "that resolve to concrete tools (e.g., pgvector, PostgreSQL). " +
                  "Each skill has a JSON Schema for input validation and supports retry policies."
)
public class SkillController {

    private final ToolInvoker toolInvoker;
    private final ToolExecutorRegistry executorRegistry;

    /**
     * Invoca uma skill com validação, retry e timeout.
     * <p>
     * O fluxo de execução:
     * 1. Resolve skill → tool concreta via backend API
     * 2. Valida input contra JSON Schema da skill
     * 3. Executa tool usando executor apropriado (HTTP, SQL, DOCUMENT_SEARCH, SCRIPT, MCP)
     * 4. Valida output contra JSON Schema da skill
     * 5. Retorna SkillResponse com resultado ou erro
     * </p>
     *
     * @param request requisição de invocação com tenantId, skillSlug, input e retry policy
     * @return {@link Mono} contendo a resposta da execução (sucesso ou erro)
     */
    @PostMapping(value = "/invoke", consumes = MediaType.APPLICATION_JSON_VALUE, produces = MediaType.APPLICATION_JSON_VALUE)
    @ResponseStatus(HttpStatus.OK)
    @Operation(
        summary = "Invoke a skill",
        description = "Executes a skill by resolving it to a concrete tool, validating inputs/outputs " +
                      "against JSON Schema, and executing with retry and timeout support. " +
                      "Supports HTTP, SQL, DOCUMENT_SEARCH, SCRIPT, and MCP tool types."
    )
    @ApiResponses({
        @ApiResponse(
            responseCode = "200",
            description = "Skill executed successfully",
            content = @Content(
                mediaType = MediaType.APPLICATION_JSON_VALUE,
                schema = @Schema(implementation = SkillResponse.class),
                examples = @ExampleObject(
                    name = "Document Search Success",
                    value = """
                    {
                      "executionId": "550e8400-e29b-41d4-a716-446655440000",
                      "skillSlug": "document-search",
                      "toolId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
                      "success": true,
                      "result": {
                        "documents": [
                          {
                            "id": "doc-123",
                            "content": "AgentHub architecture overview...",
                            "score": 0.95
                          }
                        ]
                      },
                      "error": null,
                      "metadata": {
                        "toolType": "DOCUMENT_SEARCH",
                        "executionTimeMs": 245
                      },
                      "latencyMs": 250,
                      "executedAt": "2026-03-14T10:30:00Z"
                    }
                    """
                )
            )
        ),
        @ApiResponse(
            responseCode = "400",
            description = "Invalid request (validation error, missing required fields)",
            content = @Content(
                mediaType = MediaType.APPLICATION_JSON_VALUE,
                examples = @ExampleObject(
                    name = "Validation Error",
                    value = """
                    {
                      "executionId": "550e8400-e29b-41d4-a716-446655440000",
                      "skillSlug": "document-search",
                      "toolId": null,
                      "success": false,
                      "result": {},
                      "error": "Input validation failed: $.query: is missing but it is required",
                      "metadata": {},
                      "latencyMs": 5,
                      "executedAt": "2026-03-14T10:30:00Z"
                    }
                    """
                )
            )
        ),
        @ApiResponse(
            responseCode = "404",
            description = "Skill or tool not found",
            content = @Content(
                mediaType = MediaType.APPLICATION_JSON_VALUE,
                examples = @ExampleObject(
                    name = "Skill Not Found",
                    value = """
                    {
                      "executionId": "550e8400-e29b-41d4-a716-446655440000",
                      "skillSlug": "unknown-skill",
                      "toolId": null,
                      "success": false,
                      "result": {},
                      "error": "Skill not found: unknown-skill",
                      "metadata": {},
                      "latencyMs": 12,
                      "executedAt": "2026-03-14T10:30:00Z"
                    }
                    """
                )
            )
        ),
        @ApiResponse(
            responseCode = "500",
            description = "Internal server error (tool execution failed, timeout, etc.)",
            content = @Content(
                mediaType = MediaType.APPLICATION_JSON_VALUE,
                examples = @ExampleObject(
                    name = "Execution Error",
                    value = """
                    {
                      "executionId": "550e8400-e29b-41d4-a716-446655440000",
                      "skillSlug": "document-search",
                      "toolId": null,
                      "success": false,
                      "result": {},
                      "error": "Tool execution failed after 3 retries: Connection timeout",
                      "metadata": {
                        "attempts": 3,
                        "lastError": "java.util.concurrent.TimeoutException"
                      },
                      "latencyMs": 15000,
                      "executedAt": "2026-03-14T10:30:00Z"
                    }
                    """
                )
            )
        )
    })
    @RequestBody(
        description = "Skill invocation request with tenant context, skill identification, input parameters, and optional retry policy",
        required = true,
        content = @Content(
            mediaType = MediaType.APPLICATION_JSON_VALUE,
            schema = @Schema(implementation = SkillRequest.class),
            examples = {
                @ExampleObject(
                    name = "Document Search",
                    description = "Search documents using pgvector similarity",
                    value = """
                    {
                      "tenantId": "123e4567-e89b-12d3-a456-426614174000",
                      "skillSlug": "document-search",
                      "input": {
                        "query": "What is AgentHub architecture?",
                        "limit": 5,
                        "threshold": 0.7
                      },
                      "executionContext": {
                        "agentId": "agent-001",
                        "userId": "user-001",
                        "executionId": "exec-001",
                        "nodeId": "node-search-1"
                      },
                      "timeout": 5000,
                      "retryPolicy": {
                        "maxAttempts": 3,
                        "backoffMs": 1000,
                        "backoffMultiplier": 2.0
                      }
                    }
                    """
                ),
                @ExampleObject(
                    name = "SQL Query",
                    description = "Execute SQL query with tenant isolation",
                    value = """
                    {
                      "tenantId": "123e4567-e89b-12d3-a456-426614174000",
                      "skillSlug": "sql-query",
                      "input": {
                        "sql": "SELECT * FROM agents WHERE status = :status",
                        "parameters": {
                          "status": "ACTIVE"
                        }
                      },
                      "executionContext": {
                        "agentId": "agent-002",
                        "userId": "user-002"
                      },
                      "timeout": 10000
                    }
                    """
                ),
                @ExampleObject(
                    name = "HTTP Call",
                    description = "Execute HTTP request to external API",
                    value = """
                    {
                      "tenantId": "123e4567-e89b-12d3-a456-426614174000",
                      "skillSlug": "http-get",
                      "input": {
                        "url": "https://api.example.com/data",
                        "headers": {
                          "Authorization": "Bearer token123"
                        }
                      },
                      "timeout": 3000,
                      "retryPolicy": {
                        "maxAttempts": 2,
                        "backoffMs": 500,
                        "backoffMultiplier": 1.5
                      }
                    }
                    """
                ),
                @ExampleObject(
                    name = "Groovy Script",
                    description = "Execute sandboxed Groovy script",
                    value = """
                    {
                      "tenantId": "123e4567-e89b-12d3-a456-426614174000",
                      "skillSlug": "groovy-script",
                      "input": {
                        "script": "def result = input.value * 2; return [output: result]",
                        "scriptInput": {
                          "value": 42
                        }
                      },
                      "timeout": 2000
                    }
                    """
                ),
                @ExampleObject(
                    name = "MCP Tool",
                    description = "Execute MCP (Model Context Protocol) tool via JSON-RPC 2.0",
                    value = """
                    {
                      "tenantId": "123e4567-e89b-12d3-a456-426614174000",
                      "skillSlug": "mcp-fetch",
                      "input": {
                        "method": "tools/call",
                        "params": {
                          "name": "fetch_url",
                          "arguments": {
                            "url": "https://example.com"
                          }
                        }
                      },
                      "timeout": 5000
                    }
                    """
                )
            }
        )
    )
    public Mono<SkillResponse> invokeSkill(@Valid @org.springframework.web.bind.annotation.RequestBody SkillRequest request) {
        log.info("Invoking skill: tenantId={}, skillSlug={}", request.tenantId(), request.skillSlug());
        String tenantId = request.tenantId().toString();
        TenantContext tc = new TenantContext(tenantId);
        return toolInvoker.invoke(request)
                .contextWrite(TenantContextHolder.withTenantContext(tc))
                .contextWrite(ctx -> {
                    var c = ctx.put("schema", tc.getSchemaName()).put("tenantId", tc.getTenantId());
                    return c;
                });
    }

    /**
     * Health check endpoint que retorna status do runtime e tipos de tools suportados.
     *
     * @return status UP + lista de tool types suportados
     */
    @GetMapping(value = "/health", produces = MediaType.APPLICATION_JSON_VALUE)
    @Operation(
        summary = "Health check",
        description = "Returns runtime status and list of supported tool types (HTTP, SQL, DOCUMENT_SEARCH, SCRIPT, MCP)"
    )
    @ApiResponse(
        responseCode = "200",
        description = "Runtime is healthy",
        content = @Content(
            mediaType = MediaType.APPLICATION_JSON_VALUE,
            examples = @ExampleObject(
                value = """
                {
                  "status": "UP",
                  "service": "agenthub-skill-runtime",
                  "supportedToolTypes": ["HTTP", "SQL", "DOCUMENT_SEARCH", "SCRIPT", "MCP"],
                  "timestamp": "2026-03-14T10:30:00Z"
                }
                """
            )
        )
    )
    public Mono<Map<String, Object>> health() {
        return Mono.just(Map.of(
                "status", "UP",
                "service", "agenthub-skill-runtime",
                "supportedToolTypes", executorRegistry.getSupportedTypes(),
                "timestamp", OffsetDateTime.now()
        ));
    }
}

