# AgentHub Skill Runtime

[![Java CI](https://github.com/AgentHub-Studio/agenthub-skill-runtime/actions/workflows/ci.yml/badge.svg)](https://github.com/AgentHub-Studio/agenthub-skill-runtime/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Skill and Tool Resolution Engine** para a plataforma AgentHub.

## 📋 Descrição

O **Skill Runtime** é um motor de execução reativo que resolve e executa tools com validação, retry e timeout.

### Features

- ✅ **JSON Schema Validation**: Validação automática de inputs/outputs contra schemas das skills
- ✅ **Retry Policy**: Exponential backoff configurável (até 10 tentativas)
- ✅ **Timeout Control**: Timeout por request (padrão: 30s)
- ✅ **Multi-tenancy**: Isolamento via `tenantId` header propagation
- ✅ **100% Reactive**: Spring WebFlux + R2DBC (non-blocking)
- ✅ **5 Tool Types**: HTTP, SQL, DOCUMENT_SEARCH, SCRIPT, MCP
- ✅ **OpenAPI 3.0**: Documentação completa com Swagger UI
- ✅ **Observability**: Actuator + Prometheus metrics

## 🏗️ Arquitetura

```
┌──────────────────┐
│  Orchestrator    │
└────────┬─────────┘
         │ HTTP: POST /tools/invoke
         ↓
┌─────────────────────────────┐
│   Tool Invoker              │
│   - invoke()                │
│   - invokeAsync()           │
└────────┬────────────────────┘
         │
         ├──→ Skill Resolver (resolve skill → tool)
         └──→ Tool Executor Registry

Tool Executors:
├── HttpToolExecutor (chamadas HTTP)
├── SqlToolExecutor (queries SQL)
├── DocumentSearchToolExecutor (busca semântica pgvector)
├── McpToolExecutor (integração MCP via Go runtime)
└── ScriptToolExecutor (Groovy/Python sandboxed)
```

## 🚀 Quick Start

### Pré-requisitos
- Java 21
- Maven 3.9+

### Desenvolvimento Local

```bash
git clone git@github.com:AgentHub-Studio/agenthub-skill-runtime.git
cd agenthub-skill-runtime

mvn clean install
mvn spring-boot:run
```

### Variáveis de Ambiente

| Variável | Descrição | Default |
|----------|-----------|---------|
| SERVER_PORT | Porta do servidor | 8083 |
| BACKEND_URL | URL do backend | http://localhost:8081 |
| MCP_CLIENT_URL | URL do MCP client (Go) | http://localhost:9001 |
| POSTGRES_URL | PostgreSQL para document search | jdbc:postgresql://localhost:5432/agenthub |

## 📚 API Documentation

### Swagger UI / OpenAPI

Acesse a documentação interativa completa da API:

**Local:** http://localhost:8082/swagger-ui.html  
**Docker:** http://agenthub-skill-runtime:8082/swagger-ui.html

A documentação OpenAPI inclui:
- ✅ Schemas completos de request/response
- ✅ Exemplos para todos os tool types (HTTP, SQL, DOCUMENT_SEARCH, SCRIPT, MCP)
- ✅ Try-it-out interativo
- ✅ Descrição detalhada de erros (400, 404, 500)
- ✅ JSON Schema validation examples

### API Endpoints

#### POST /api/v1/skills/invoke
Invoca uma skill com validação, retry e timeout.

**Request:**
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

**Response (Success):**
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

**Response (Error):**
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

#### GET /api/v1/skills/health
Health check endpoint.

**Response:**
```json
{
  "status": "UP",
  "service": "agenthub-skill-runtime",
  "supportedToolTypes": ["HTTP", "SQL", "DOCUMENT_SEARCH", "SCRIPT", "MCP"],
  "timestamp": "2026-03-14T10:30:00Z"
}
```

## 🧪 Testes

```bash
mvn test
mvn clean test jacoco:report
```

## 📖 Documentação

- [Skills e Tools](https://github.com/AgentHub-Studio/agenthub-middleware/blob/main/docs/spec/AGENTS.md)
- [Roadmap](https://github.com/AgentHub-Studio/agenthub-middleware/blob/main/docs/SPRINTS_V2.md)

## 📝 Licença

MIT License
