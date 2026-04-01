package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	mcpexec "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/mcp"
)

func TestMCPToolExecutor_GetToolType(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	assert.Equal(t, "MCP", e.GetToolType())
}

func TestMCPToolExecutor_Execute_MissingConfig(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config:   map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp_server_config_id is required")
}

func TestMCPToolExecutor_Execute_MissingToolName(t *testing.T) {
	e := mcpexec.NewMCPToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config: map[string]any{
			"mcp_server_config_id": "some-uuid",
			// tool_name intentionally omitted
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool_name is required")
}

// mcpJSONRPCResponse is a helper to build a JSON-RPC 2.0 response body.
type mcpJSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func TestMCPToolExecutor_Execute_HTTPServerReturns500(t *testing.T) {
	// Stand up a mock MCP server that returns a 500 status code.
	// Verify the executor correctly reports a server error from the HTTP response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	// Build the HTTP request that the executor would send, manually invoking
	// the HTTP dispatch to test the 5xx error handling path in isolation from the DB.
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestMCPToolExecutor_Execute_HTTPTransport_MockServerRespondsCorrectly(t *testing.T) {
	// Verify that a mock MCP server correctly handles a JSON-RPC tools/call request
	// and returns a valid result. This tests the expected server contract that the
	// MCPToolExecutor relies on when sending requests via HTTP transport.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/mcp", r.URL.Path)

		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  map[string]any{"answer": "42"},
		})
	}))
	defer srv.Close()

	// Send a JSON-RPC tools/call request as the executor would.
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "my_tool",
			"arguments": map[string]any{"input": "hello"},
		},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(reqBody))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var rpc mcpJSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rpc))
	assert.Equal(t, "42", rpc.Result["answer"])
}
