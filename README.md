# AgentHub Skill Runtime

[![Java CI](https://github.com/AgentHub-Studio/agenthub-skill-runtime/actions/workflows/ci.yml/badge.svg)](https://github.com/AgentHub-Studio/agenthub-skill-runtime/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Skill and Tool Resolution Engine** para a plataforma AgentHub.

## 📋 Descrição

O **Skill Runtime** é responsável por:
- Resolver skills para implementações concretas (tools)
- Executar tools (HTTP, SQL, Document Search, MCP, etc.)
- Gerenciar tool executors
- Validar e sanitizar inputs/outputs
- Registrar execuções de tools

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

## 📚 API Principal

### POST /api/v1/tools/invoke
Invoca uma tool.

**Request:**
```json
{
  "tenantId": "uuid",
  "skillSlug": "document-search",
  "input": {
    "query": "contratos de 2025",
    "knowledgeBaseId": "uuid",
    "limit": 10
  }
}
```

**Response:**
```json
{
  "executionId": "uuid",
  "success": true,
  "result": {
    "documents": [
      {
        "content": "...",
        "score": 0.95
      }
    ]
  },
  "latencyMs": 150
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
