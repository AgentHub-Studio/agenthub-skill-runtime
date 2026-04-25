package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// MCPToolExecutor invokes a skill via the MCP protocol over HTTP transport.
//
// Config fields:
//   - mcp_server_config_id: string (UUID of mcp_server_config)
//   - tool_name: string (MCP tool name to invoke)
//
// Only the "http" transport type is supported; "stdio" returns an error.
type MCPToolExecutor struct {
	pool *pgxpool.Pool
}

// NewMCPToolExecutor creates an MCPToolExecutor backed by pool.
func NewMCPToolExecutor(pool *pgxpool.Pool) *MCPToolExecutor {
	return &MCPToolExecutor{pool: pool}
}

// GetToolType returns the tool type identifier.
func (e *MCPToolExecutor) GetToolType() string { return "MCP" }

// mcpToolConfig holds parsed configuration for an MCP tool.
type mcpToolConfig struct {
	MCPServerConfigID string `json:"mcp_server_config_id"`
	ToolName          string `json:"tool_name"`
}

// mcpServerConfig represents a row from ah_{tenantID}.mcp_server_config.
type mcpServerConfig struct {
	TransportType string
	HTTPBaseURL   string
}

// mcpRequest is a JSON-RPC 2.0 request to the MCP server.
type mcpRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

// mcpResponse is a JSON-RPC 2.0 response from the MCP server.
type mcpResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *mcpError      `json:"error,omitempty"`
}

// mcpError represents a JSON-RPC error object.
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Execute fetches the MCP server config and invokes the tool via JSON-RPC over HTTP.
func (e *MCPToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	cfg, err := parseMCPToolConfig(ec.Config)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: parse config: %w", err)
	}

	if cfg.MCPServerConfigID == "" {
		return nil, fmt.Errorf("mcp executor: mcp_server_config_id is required")
	}
	if cfg.ToolName == "" {
		return nil, fmt.Errorf("mcp executor: tool_name is required")
	}

	serverCfg, err := e.fetchServerConfig(ctx, ec.TenantID, cfg.MCPServerConfigID)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: fetch server config: %w", err)
	}

	if serverCfg.TransportType == "stdio" {
		return nil, fmt.Errorf("mcp executor: stdio transport not supported in runtime")
	}
	if serverCfg.HTTPBaseURL == "" {
		return nil, fmt.Errorf("mcp executor: http_base_url is required for http transport")
	}

	rpcReq := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      cfg.ToolName,
			"arguments": ec.Input,
		},
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, serverCfg.HTTPBaseURL+"/mcp", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcp executor: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	rawBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp executor: server returned %d: %s", httpResp.StatusCode, string(rawBody))
	}

	var rpcResp mcpResponse
	if err := json.Unmarshal(rawBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("mcp executor: decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return &executor.Result{Error: fmt.Sprintf("mcp error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)}, nil
	}

	return &executor.Result{Output: rpcResp.Result}, nil
}

// fetchServerConfig loads the MCP server configuration from the tenant schema.
func (e *MCPToolExecutor) fetchServerConfig(ctx context.Context, tenantID, serverConfigID string) (*mcpServerConfig, error) {
	schema := executor.TenantSchema(tenantID)
	query := fmt.Sprintf(
		`SELECT transport_type, COALESCE(http_base_url, '') FROM %s.mcp_server_config WHERE id = $1`,
		schema,
	)

	var cfg mcpServerConfig
	row := e.pool.QueryRow(ctx, query, serverConfigID)
	if err := row.Scan(&cfg.TransportType, &cfg.HTTPBaseURL); err != nil {
		return nil, fmt.Errorf("mcp_server_config %q not found in tenant %q: %w", serverConfigID, tenantID, err)
	}
	return &cfg, nil
}

// parseMCPToolConfig decodes the executor config map.
func parseMCPToolConfig(raw map[string]any) (*mcpToolConfig, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var cfg mcpToolConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
