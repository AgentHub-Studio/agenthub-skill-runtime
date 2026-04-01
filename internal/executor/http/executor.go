package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// HTTPToolExecutor executes HTTP tool calls using URL templates.
//
// Config fields (from tool.config JSONB):
//   - url: string (supports {{input.fieldName}} templates)
//   - method: string (GET, POST, PUT, DELETE, PATCH)
//   - headers: map[string]string (supports {{input.fieldName}} templates)
//   - body_template: string (JSON template with {{input.fieldName}})
//   - timeout_seconds: int (default 30)
//   - auth_type: string (none, bearer, basic, oauth2)
//   - auth_token: string (for bearer/basic)
type HTTPToolExecutor struct{}

// GetToolType returns the tool type identifier.
func (e *HTTPToolExecutor) GetToolType() string { return "HTTP" }

// httpConfig holds parsed configuration for an HTTP tool.
type httpConfig struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	BodyTemplate   string            `json:"body_template"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	AuthType       string            `json:"auth_type"`
	AuthToken      string            `json:"auth_token"`
}

// Execute performs the HTTP request described in ec.Config using values from ec.Input.
func (e *HTTPToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	cfg, err := parseHTTPConfig(ec.Config)
	if err != nil {
		return nil, fmt.Errorf("http executor: parse config: %w", err)
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("http executor: url is required")
	}

	method := strings.ToUpper(cfg.Method)
	if method == "" {
		method = http.MethodGet
	}

	renderedURL := renderTemplate(cfg.URL, ec.Input)

	timeoutSeconds := cfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if cfg.BodyTemplate != "" {
		rendered := renderTemplate(cfg.BodyTemplate, ec.Input)
		bodyReader = strings.NewReader(rendered)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, renderedURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http executor: build request: %w", err)
	}

	for k, v := range cfg.Headers {
		req.Header.Set(k, renderTemplate(v, ec.Input))
	}

	// Set Content-Type for requests with a body if not already set.
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply authentication.
	switch strings.ToLower(cfg.AuthType) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	case "basic":
		req.Header.Set("Authorization", "Basic "+cfg.AuthToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http executor: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http executor: read response body: %w", err)
	}

	// Attempt to parse response as JSON; fall back to raw string.
	var responsePayload any
	if err := json.Unmarshal(respBytes, &responsePayload); err != nil {
		responsePayload = string(respBytes)
	}

	output := map[string]any{
		"response":    responsePayload,
		"status_code": resp.StatusCode,
	}

	return &executor.Result{Output: output}, nil
}

// parseHTTPConfig decodes the executor config map into an httpConfig struct.
func parseHTTPConfig(raw map[string]any) (*httpConfig, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var cfg httpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// renderTemplate replaces all {{input.key}} placeholders in tmpl with the
// corresponding string values from input.
func renderTemplate(tmpl string, input map[string]any) string {
	pairs := make([]string, 0, len(input)*2)
	for k, v := range input {
		pairs = append(pairs, fmt.Sprintf("{{input.%s}}", k), fmt.Sprintf("%v", v))
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
