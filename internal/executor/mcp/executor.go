package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	httpexec "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/http"
)

const (
	maxStdioRPCMessageSize     = 4 * 1024 * 1024
	mcpStdioAllowedCommandsEnv = "MCP_STDIO_ALLOWED_COMMANDS"
	defaultStdioCommandPath    = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

// MCPToolExecutor invokes a skill through a configured MCP server.
// Both stdio and Streamable HTTP configurations are supported.
type MCPToolExecutor struct {
	pool                  *pgxpool.Pool
	stdioCommandAllowlist map[string]string
	stdioCommandConfigErr error
	resolveHost           func(context.Context, string) ([]net.IPAddr, error)
	dialContext           func(context.Context, string, string) (net.Conn, error)
}

// NewMCPToolExecutor creates an MCPToolExecutor backed by pool.
func NewMCPToolExecutor(pool *pgxpool.Pool) *MCPToolExecutor {
	allowlist, err := parseStdioCommandAllowlist(os.Getenv(mcpStdioAllowedCommandsEnv))
	dialer := &net.Dialer{}
	return &MCPToolExecutor{
		pool:                  pool,
		stdioCommandAllowlist: allowlist,
		stdioCommandConfigErr: err,
		resolveHost:           net.DefaultResolver.LookupIPAddr,
		dialContext:           dialer.DialContext,
	}
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
	Command       string
	Args          []string
	Env           map[string]string
}

// mcpRequest is a JSON-RPC 2.0 request to the MCP server.
type mcpRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// mcpResponse is a JSON-RPC 2.0 response from the MCP server.
type mcpResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *mcpError      `json:"error,omitempty"`
}

// mcpError represents a JSON-RPC error object.
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Execute fetches the MCP server config and invokes the configured tool.
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

	switch serverCfg.TransportType {
	case "stdio":
		if e.stdioCommandConfigErr != nil {
			return nil, fmt.Errorf("mcp executor: invalid %s: %w", mcpStdioAllowedCommandsEnv, e.stdioCommandConfigErr)
		}
		return executeStdio(ctx, serverCfg, cfg.ToolName, ec.Input, e.stdioCommandAllowlist)
	case "http":
		return e.executeHTTP(ctx, serverCfg, cfg.ToolName, ec.Input)
	default:
		return nil, fmt.Errorf("mcp executor: unsupported transport type %q", serverCfg.TransportType)
	}
}

func (e *MCPToolExecutor) executeHTTP(ctx context.Context, serverCfg *mcpServerConfig, toolName string, input map[string]any) (*executor.Result, error) {
	if serverCfg.HTTPBaseURL == "" {
		return nil, fmt.Errorf("mcp executor: http_base_url is required for http transport")
	}
	endpoint := strings.TrimRight(serverCfg.HTTPBaseURL, "/") + "/mcp"
	if err := httpexec.ValidateURL(endpoint); err != nil {
		return nil, fmt.Errorf("mcp executor: blocked HTTP endpoint: %w", err)
	}

	rpcReq := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      toolName,
			"arguments": input,
		},
	}
	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcp executor: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := e.newProtectedHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: HTTP request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

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
	return resultFromResponse(&rpcResp), nil
}

func (e *MCPToolExecutor) newProtectedHTTPClient() *http.Client {
	transport := &http.Transport{
		// Do not honor HTTP(S)_PROXY: proxy egress bypasses destination pinning.
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return e.dialHTTPOutbound(ctx, network, address)
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("mcp executor: too many HTTP redirects")
			}
			if err := httpexec.ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("mcp executor: redirect blocked: %w", err)
			}
			return nil
		},
	}
}

// dialHTTPOutbound resolves each configured MCP HTTP host exactly once, rejects
// unsafe addresses, then dials the selected IP directly to prevent DNS rebinding.
func (e *MCPToolExecutor) dialHTTPOutbound(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: invalid HTTP outbound address: %w", err)
	}
	if ip := net.ParseIP(host); ip != nil {
		if httpexec.IsBlockedOutboundIP(ip) {
			return nil, fmt.Errorf("mcp executor: HTTP outbound target is blocked: %s", ip)
		}
		return e.dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	addresses, err := e.resolveHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: resolve HTTP outbound host %q: %w", host, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("mcp executor: HTTP outbound host %q resolved without addresses", host)
	}
	for _, resolved := range addresses {
		if httpexec.IsBlockedOutboundIP(resolved.IP) {
			return nil, fmt.Errorf("mcp executor: HTTP outbound host %q resolves to blocked address %s", host, resolved.IP)
		}
	}
	return e.dialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func executeStdio(ctx context.Context, serverCfg *mcpServerConfig, toolName string, input map[string]any, commandAllowlist map[string]string) (*executor.Result, error) {
	if strings.TrimSpace(serverCfg.Command) == "" {
		return nil, fmt.Errorf("mcp executor: command is required for stdio transport")
	}
	command, err := resolveStdioCommand(serverCfg.Command, commandAllowlist)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: stdio command rejected: %w", err)
	}
	environment, err := stdioEnvironment(serverCfg.Env)
	if err != nil {
		return nil, fmt.Errorf("mcp executor: invalid stdio environment: %w", err)
	}

	// #nosec G204 -- command is an exact match from the deployment-owned allowlist.
	cmd := exec.CommandContext(ctx, command, serverCfg.Args...)
	cmd.Env = environment
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp executor: open stdio stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp executor: open stdio stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp executor: start stdio command %q: %w", command, err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	protocol := newStdioProtocol(stdout, stdin)
	initialize, err := protocol.request(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "agenthub-skill-runtime",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mcp executor: stdio initialize: %w", err)
	}
	if initialize.Error != nil {
		return nil, fmt.Errorf("mcp executor: stdio initialize error %d: %s", initialize.Error.Code, initialize.Error.Message)
	}
	if err := protocol.notification("notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("mcp executor: stdio initialized notification: %w", err)
	}

	response, err := protocol.request(ctx, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": input,
	})
	if err != nil {
		return nil, fmt.Errorf("mcp executor: stdio tools/call: %w", err)
	}
	return resultFromResponse(response), nil
}

func resultFromResponse(response *mcpResponse) *executor.Result {
	if response.Error != nil {
		return &executor.Result{Error: fmt.Sprintf("mcp error %d: %s", response.Error.Code, response.Error.Message)}
	}
	return &executor.Result{Output: response.Result}
}

type stdioProtocol struct {
	reader *bufio.Scanner
	writer io.Writer
	nextID int
}

func newStdioProtocol(reader io.Reader, writer io.Writer) *stdioProtocol {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), maxStdioRPCMessageSize)
	return &stdioProtocol{reader: scanner, writer: writer}
}

func (p *stdioProtocol) request(ctx context.Context, method string, params map[string]any) (*mcpResponse, error) {
	p.nextID++
	id := p.nextID
	if err := p.write(mcpRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}

	for p.reader.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var response mcpResponse
		if err := json.Unmarshal(p.reader.Bytes(), &response); err != nil {
			return nil, fmt.Errorf("decode stdio response: %w", err)
		}
		if response.ID == id {
			return &response, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.reader.Err(); err != nil {
		return nil, fmt.Errorf("read stdio response: %w", err)
	}
	return nil, fmt.Errorf("stdio MCP server closed before responding")
}

func (p *stdioProtocol) notification(method string, params map[string]any) error {
	return p.write(mcpRequest{JSONRPC: "2.0", Method: method, Params: params})
}

func (p *stdioProtocol) write(request mcpRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal stdio request: %w", err)
	}
	data = append(data, '\n')
	if _, err := p.writer.Write(data); err != nil {
		return fmt.Errorf("write stdio request: %w", err)
	}
	return nil
}

func stdioEnvironment(values map[string]string) ([]string, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	entries := make([]string, 1, len(keys)+1)
	entries[0] = "PATH=" + defaultStdioCommandPath
	for _, key := range keys {
		if !isPOSIXEnvName(key) || isProtectedStdioEnvName(key) || strings.ContainsRune(values[key], rune(0)) {
			return nil, fmt.Errorf("%q", key)
		}
		entries = append(entries, key+"="+values[key])
	}
	return entries, nil
}

func isProtectedStdioEnvName(name string) bool {
	normalized := strings.ToUpper(name)
	if normalized == "PATH" || normalized == "ENV" || normalized == "BASH_ENV" || normalized == "NODE_OPTIONS" || normalized == "RUBYOPT" || normalized == "PERL5OPT" || normalized == "GODEBUG" {
		return true
	}
	return strings.HasPrefix(normalized, "LD_") || strings.HasPrefix(normalized, "DYLD_") || strings.HasPrefix(normalized, "PYTHON")
}

func parseStdioCommandAllowlist(raw string) (map[string]string, error) {
	allowlist := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return allowlist, nil
	}

	for _, entry := range strings.Split(raw, ",") {
		configured := filepath.Clean(strings.TrimSpace(entry))
		if !filepath.IsAbs(configured) {
			return nil, fmt.Errorf("command %q must be an absolute path", entry)
		}
		resolved, err := filepath.EvalSymlinks(configured)
		if err != nil {
			return nil, fmt.Errorf("command %q: %w", configured, err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("command %q: %w", resolved, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return nil, fmt.Errorf("command %q is not an executable regular file", configured)
		}
		allowlist[configured] = resolved
	}
	return allowlist, nil
}

func resolveStdioCommand(command string, allowlist map[string]string) (string, error) {
	configured := filepath.Clean(strings.TrimSpace(command))
	if !filepath.IsAbs(configured) {
		return "", fmt.Errorf("command must be an absolute path")
	}
	resolved, ok := allowlist[configured]
	if !ok {
		return "", fmt.Errorf("command is not in the deployment allowlist")
	}
	return resolved, nil
}

func isPOSIXEnvName(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}

// fetchServerConfig loads the MCP server configuration from the tenant schema.
func (e *MCPToolExecutor) fetchServerConfig(ctx context.Context, tenantID, serverConfigID string) (*mcpServerConfig, error) {
	schema := executor.TenantSchema(tenantID)
	query := fmt.Sprintf(
		`SELECT transport_type, COALESCE(http_base_url, ''), COALESCE(command, ''),
			COALESCE(args, '[]'::jsonb), COALESCE(env, '{}'::jsonb)
		 FROM %s.mcp_server_config WHERE id = $1`,
		schema,
	)

	var cfg mcpServerConfig
	var argsJSON, envJSON []byte
	row := e.pool.QueryRow(ctx, query, serverConfigID)
	if err := row.Scan(&cfg.TransportType, &cfg.HTTPBaseURL, &cfg.Command, &argsJSON, &envJSON); err != nil {
		return nil, fmt.Errorf("mcp_server_config %q not found in tenant %q: %w", serverConfigID, tenantID, err)
	}
	if err := json.Unmarshal(argsJSON, &cfg.Args); err != nil {
		return nil, fmt.Errorf("decode MCP args: %w", err)
	}
	if err := json.Unmarshal(envJSON, &cfg.Env); err != nil {
		return nil, fmt.Errorf("decode MCP env: %w", err)
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
