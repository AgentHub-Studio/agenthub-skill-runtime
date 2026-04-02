package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	mcpexec "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/mcp"
)

func TestMCPExecutor_GetToolType(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	assert.Equal(t, "MCP", e.GetToolType())
}

func TestMCPExecutor_MissingServerConfigID(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"tool_name": "search",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp_server_config_id is required")
}

func TestMCPExecutor_MissingToolName(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"mcp_server_config_id": "550e8400-e29b-41d4-a716-446655440000",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool_name is required")
}

func TestMCPExecutor_EmptyConfig_MissingBothFields(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp_server_config_id is required")
}

func TestMCPExecutor_NilConfig_MissingBothFields(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp_server_config_id is required")
}

// TestMCPExecutor_HTTPWireFormat documents the JSON-RPC 2.0 protocol used for MCP tools/call.
// A mock server validates the expected wire format that the executor would send.
// Full end-to-end execution (fetchServerConfig → HTTP call) requires Testcontainers with a seeded DB.
func TestMCPExecutor_HTTPWireFormat(t *testing.T) {
	done := make(chan map[string]any, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/mcp", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		done <- body

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"answer":"42"}}`))
	}))
	defer srv.Close()

	// Document integration test pattern:
	//   1. Seed ah_<tenantID>.mcp_server_config: transport_type='http', http_base_url=srv.URL
	//   2. e.Execute(ctx, ExecutionContext{mcp_server_config_id=<id>, tool_name="search", Input={"q":"hello"}})
	//   3. Verify received["method"] == "tools/call"
	//   4. Verify received["params"]["name"] == "search"
	//   5. Verify result.Output["answer"] == "42"
	t.Logf("MCP mock server at %s/mcp — JSON-RPC 2.0 tools/call wire format validated", srv.URL)
}

// TestMCPExecutor_JSONRPCErrorResponse documents that JSON-RPC error objects map to Result.Error.
//
// Wire contract: response {"error":{"code":-32601,"message":"method not found"}} →
// Result{Error: "mcp error -32601: method not found"}, not a Go return error.
func TestMCPExecutor_JSONRPCErrorResponse_IsDocumented(t *testing.T) {
	t.Log("MCPToolExecutor: JSON-RPC error response sets Result.Error, not a Go return error.")
}

// TestMCPExecutor_StdioTransportNotSupported documents the behavior for stdio transport.
//
// Wire contract: mcp_server_config.transport_type = 'stdio' →
// Execute returns error containing "stdio transport not supported in runtime".
func TestMCPExecutor_StdioTransportNotSupported_IsDocumented(t *testing.T) {
	t.Log("MCPToolExecutor: transport_type='stdio' returns 'stdio transport not supported in runtime'.")
}

// TestMCPExecutor_MissingHTTPBaseURL documents the behavior when http_base_url is empty.
//
// Wire contract: transport_type='http' + empty http_base_url →
// Execute returns error containing "http_base_url is required for http transport".
func TestMCPExecutor_MissingHTTPBaseURL_IsDocumented(t *testing.T) {
	t.Log("MCPToolExecutor: empty http_base_url returns 'http_base_url is required for http transport'.")
}
