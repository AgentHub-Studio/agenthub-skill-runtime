# Swagger UI Examples

Este documento contém exemplos de uso do Swagger UI para testar o AgentHub Skill Runtime.

## Acesso

**Local:** http://localhost:8082/swagger-ui.html  
**OpenAPI JSON:** http://localhost:8082/api-docs

## Exemplos de Request

### 1. Document Search (pgvector)

Busca semântica em documentos usando embeddings.

```json
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
```

### 2. SQL Query

Executa query SQL com tenant isolation automático.

```json
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
```

### 3. HTTP GET

Executa HTTP GET request com headers customizados.

```json
{
  "tenantId": "123e4567-e89b-12d3-a456-426614174000",
  "skillSlug": "http-get",
  "input": {
    "url": "https://api.example.com/data",
    "headers": {
      "Authorization": "Bearer token123",
      "Accept": "application/json"
    }
  },
  "timeout": 3000,
  "retryPolicy": {
    "maxAttempts": 2,
    "backoffMs": 500,
    "backoffMultiplier": 1.5
  }
}
```

### 4. HTTP POST

Executa HTTP POST com body JSON.

```json
{
  "tenantId": "123e4567-e89b-12d3-a456-426614174000",
  "skillSlug": "http-post",
  "input": {
    "url": "https://api.example.com/webhooks",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    },
    "body": {
      "event": "agent.completed",
      "agentId": "agent-001",
      "result": "success"
    }
  },
  "timeout": 5000
}
```

### 5. Groovy Script

Executa script Groovy sandboxed.

```json
{
  "tenantId": "123e4567-e89b-12d3-a456-426614174000",
  "skillSlug": "groovy-script",
  "input": {
    "script": "def result = input.value * 2\nreturn [output: result, computed: true]",
    "scriptInput": {
      "value": 42
    }
  },
  "timeout": 2000
}
```

### 6. MCP Tool

Executa tool via Model Context Protocol (JSON-RPC 2.0).

```json
{
  "tenantId": "123e4567-e89b-12d3-a456-426614174000",
  "skillSlug": "mcp-fetch",
  "input": {
    "method": "tools/call",
    "params": {
      "name": "fetch_url",
      "arguments": {
        "url": "https://example.com",
        "format": "markdown"
      }
    }
  },
  "timeout": 5000
}
```

## Responses Esperados

### Success (200)

```json
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
```

### Validation Error (400)

```json
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
```

### Not Found (404)

```json
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
```

### Execution Error (500)

```json
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
```

## Testando no Swagger UI

1. **Abra o Swagger UI:** http://localhost:8082/swagger-ui.html
2. **Expanda o endpoint:** `POST /api/v1/skills/invoke`
3. **Clique em "Try it out"**
4. **Cole um dos exemplos acima** no campo "Request body"
5. **Clique em "Execute"**
6. **Verifique a resposta** na seção "Responses"

## JSON Schema Validation

O runtime valida automaticamente inputs/outputs contra os schemas das skills.

### Exemplo de Schema

```json
{
  "inputSchema": {
    "type": "object",
    "required": ["query"],
    "properties": {
      "query": {
        "type": "string",
        "minLength": 1,
        "description": "Search query"
      },
      "limit": {
        "type": "integer",
        "minimum": 1,
        "maximum": 100,
        "default": 10
      },
      "threshold": {
        "type": "number",
        "minimum": 0.0,
        "maximum": 1.0,
        "default": 0.7
      }
    }
  }
}
```

### Erros de Validação

Se o input violar o schema, você receberá um erro detalhado:

```json
{
  "error": "Input validation failed: $.query: is missing but it is required"
}
```

```json
{
  "error": "Input validation failed: $.limit: must have a maximum value of 100"
}
```

## Health Check

Use o endpoint `/health` para verificar o status do runtime:

```bash
curl http://localhost:8082/api/v1/skills/health
```

Resposta:

```json
{
  "status": "UP",
  "service": "agenthub-skill-runtime",
  "supportedToolTypes": ["HTTP", "SQL", "DOCUMENT_SEARCH", "SCRIPT", "MCP"],
  "timestamp": "2026-03-14T10:30:00Z"
}
```

## Retry Policy

Configure retry com exponential backoff:

```json
{
  "retryPolicy": {
    "maxAttempts": 3,       // Número de retries (não conta tentativa inicial)
    "backoffMs": 1000,      // Delay inicial em ms
    "backoffMultiplier": 2.0 // Multiplicador (2.0 = dobra cada retry)
  }
}
```

Tentativas:
1. Tentativa inicial: 0ms
2. Retry 1: +1000ms (1s)
3. Retry 2: +2000ms (2s)
4. Retry 3: +4000ms (4s)

Total: 4 tentativas, 7s de delay acumulado
