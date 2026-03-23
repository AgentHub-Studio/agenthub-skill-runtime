package dev.cezar.agenthub.skillruntime.adapter.config;

import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Contact;
import io.swagger.v3.oas.models.info.Info;
import io.swagger.v3.oas.models.info.License;
import io.swagger.v3.oas.models.servers.Server;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.util.List;

/**
 * Configuração OpenAPI 3.0 para documentação da API.
 *
 * @since 1.0.0
 */
@Configuration
public class OpenApiConfig {

    @Bean
    public OpenAPI skillRuntimeOpenAPI() {
        return new OpenAPI()
                .info(new Info()
                        .title("AgentHub Skill Runtime API")
                        .description("""
                                **AgentHub Skill Runtime** is a reactive execution engine that resolves and executes tools with validation, retry, and timeout support.
                                
                                ## Architecture
                                
                                - **Skills**: Abstract capabilities (e.g., "document-search", "send-email")
                                - **Tools**: Concrete implementations (e.g., pgvector, SendGrid, AWS SES)
                                - **Executors**: Type-specific execution engines (HTTP, SQL, DOCUMENT_SEARCH, SCRIPT, MCP)
                                
                                ## Features
                                
                                - ✅ **JSON Schema Validation**: Automatic input/output validation against skill schemas
                                - ✅ **Retry Policy**: Configurable exponential backoff retry (max 10 attempts)
                                - ✅ **Timeout Control**: Per-request timeout configuration (default: 30s)
                                - ✅ **Multi-tenancy**: Tenant isolation via `tenantId` header propagation
                                - ✅ **Reactive**: 100% non-blocking with Spring WebFlux + R2DBC
                                - ✅ **Tool Types**: HTTP, SQL, DOCUMENT_SEARCH (pgvector), SCRIPT (Groovy), MCP (JSON-RPC 2.0)
                                
                                ## Tool Types
                                
                                | Type | Description | Executor |
                                |------|-------------|----------|
                                | **HTTP** | REST API calls (GET, POST, PUT, DELETE, PATCH) | `HttpToolExecutor` |
                                | **SQL** | Database queries with R2DBC (PostgreSQL) | `SqlToolExecutor` |
                                | **DOCUMENT_SEARCH** | Vector similarity search (pgvector + embeddings) | `DocumentSearchToolExecutor` |
                                | **SCRIPT** | Sandboxed Groovy script execution | `ScriptToolExecutor` |
                                | **MCP** | Model Context Protocol via JSON-RPC 2.0 | `McpToolExecutor` |
                                
                                ## Workflow
                                
                                1. **Resolve**: Skill slug → Active tool with lowest priority (backend API)
                                2. **Validate Input**: Input against skill's `inputSchema` (JSON Schema Draft 2020-12)
                                3. **Execute**: Tool via type-specific executor with retry + timeout
                                4. **Validate Output**: Output against skill's `outputSchema`
                                5. **Return**: `SkillResponse` with result or error
                                
                                ## Error Handling
                                
                                - **400**: Validation errors (missing required fields, type mismatch, constraint violations)
                                - **404**: Skill/tool not found
                                - **500**: Execution errors (timeout, connection failure, script errors)
                                - **Retry**: Automatic retry with exponential backoff on transient failures
                                
                                ## Authentication
                                
                                - Header: `X-Tenant-ID` (required for all requests)
                                - Tenant context propagated to all backend/tool calls
                                """)
                        .version("1.0.0")
                        .contact(new Contact()
                                .name("AgentHub Team")
                                .email("dev@agenthub.dev")
                                .url("https://github.com/cezardev/agenthub"))
                        .license(new License()
                                .name("MIT License")
                                .url("https://opensource.org/licenses/MIT")))
                .servers(List.of(
                        new Server()
                                .url("http://localhost:8082")
                                .description("Local Development"),
                        new Server()
                                .url("http://agenthub-skill-runtime:8082")
                                .description("Docker Internal Network"),
                        new Server()
                                .url("https://api.agenthub.dev")
                                .description("Production API")
                ));
    }
}
