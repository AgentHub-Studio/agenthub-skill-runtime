package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// HTTPToolExecutor executes HTTP tool calls using URL templates.
//
// Config fields (from tool.config JSONB):
//   - url: string (supports {key} and {{input.key}} templates)
//   - method: string (GET, POST, PUT, DELETE, PATCH)
//   - headers: map[string]string
//   - body_template: string (JSON template with {key} or {{input.key}})
//   - timeout_seconds: int (default 30)
//   - auth_type: string (none, bearer, basic)
//   - auth_token: string (static token for bearer/basic)
//   - useCallerToken: bool (forward the caller's JWT as Authorization: Bearer)
type HTTPToolExecutor struct {
	// backendBaseURL is prepended to relative URLs (starting with /).
	backendBaseURL string
	// urlValidator is the SSRF guard applied before every outbound request.
	// When nil, ValidateURL (the production SSRF guard) is used.
	// Override for testing via WithURLValidator.
	urlValidator func(string) error
}

// NewHTTPToolExecutor creates an HTTPToolExecutor with the given backend base URL.
// The base URL is used for relative tool URLs (e.g. /api/skills → http://agenthub-api:8081/api/skills).
func NewHTTPToolExecutor(backendBaseURL string) *HTTPToolExecutor {
	return &HTTPToolExecutor{backendBaseURL: backendBaseURL}
}

// WithURLValidator overrides the SSRF URL validator. Intended for testing only.
// Pass nil to restore the default production ValidateURL guard.
func (e *HTTPToolExecutor) WithURLValidator(fn func(string) error) *HTTPToolExecutor {
	e.urlValidator = fn
	return e
}

// GetToolType returns the tool type identifier.
func (e *HTTPToolExecutor) GetToolType() string { return "HTTP" }

// httpConfig holds parsed configuration for an HTTP tool.
type httpConfig struct {
	URL             string            `json:"url"`
	URLTemplate     string            `json:"urlTemplate"` // alias used by some tool configs
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	BodyTemplate    string            `json:"body_template"`
	TimeoutSeconds  int               `json:"timeout_seconds"`
	AuthType        string            `json:"auth_type"`
	AuthToken       string            `json:"auth_token"`
	UseCallerToken  bool              `json:"useCallerToken"`
	BaseURL         string            `json:"baseUrl"` // optional base URL prefix
}

// Execute performs the HTTP request described in ec.Config using values from ec.Input.
func (e *HTTPToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	cfg, err := parseHTTPConfig(ec.Config)
	if err != nil {
		return nil, fmt.Errorf("http executor: parse config: %w", err)
	}

	// Resolve URL: prefer cfg.URL, fallback to cfg.URLTemplate.
	rawURL := cfg.URL
	if rawURL == "" {
		rawURL = cfg.URLTemplate
	}
	if rawURL == "" {
		return nil, fmt.Errorf("http executor: url is required")
	}
	// Prepend base URL for relative paths: use tool config baseUrl first, then executor default.
	if !strings.HasPrefix(rawURL, "http") {
		base := cfg.BaseURL
		if base == "" {
			base = e.backendBaseURL
		}
		if base != "" {
			rawURL = strings.TrimRight(base, "/") + rawURL
		}
	}

	method := strings.ToUpper(cfg.Method)
	if method == "" {
		method = http.MethodGet
	}

	renderedURL := renderTemplateURL(rawURL, ec.Input)

	// P-C220-1 / P-C221-1: reject SSRF attempts before sending any request.
	validate := e.urlValidator
	if validate == nil {
		validate = ValidateURL // production default
	}
	if err := validate(renderedURL); err != nil {
		return nil, fmt.Errorf("http executor: blocked URL (%w)", err)
	}

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
	switch {
	case cfg.UseCallerToken && ec.CallerToken != "":
		req.Header.Set("Authorization", "Bearer "+ec.CallerToken)
	case strings.EqualFold(cfg.AuthType, "bearer") && cfg.AuthToken != "":
		req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	case strings.EqualFold(cfg.AuthType, "basic") && cfg.AuthToken != "":
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

// renderTemplate replaces placeholders in tmpl with corresponding string values
// from input. Supports three formats (matched in priority order to avoid
// partial substitution of double-brace templates):
//   - {{key}}         — Handlebars/Mustache-style (most intuitive for users)
//   - {key}           — single-brace format stored in the database
//   - {{input.key}}   — explicit input-namespace format
func renderTemplate(tmpl string, input map[string]any) string {
	pairs := make([]string, 0, len(input)*6)
	for k, v := range input {
		val := fmt.Sprintf("%v", v)
		// {{key}} must come BEFORE {key} so that double-brace templates are
		// matched first; otherwise {key} inside {{key}} would be replaced first,
		// producing {value} (with stray curly braces) instead of value.
		pairs = append(pairs,
			fmt.Sprintf("{{%s}}", k), val,
			fmt.Sprintf("{%s}", k), val,
			fmt.Sprintf("{{input.%s}}", k), val,
		)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

// renderTemplateURL is like renderTemplate but URL-encodes each substituted value.
// P-C285-1: prevents malformed URLs when input values contain spaces, &, =, +, etc.
// Should be used only for URL rendering, NOT for body/header templates.
func renderTemplateURL(tmpl string, input map[string]any) string {
	pairs := make([]string, 0, len(input)*6)
	for k, v := range input {
		encoded := url.QueryEscape(fmt.Sprintf("%v", v))
		pairs = append(pairs,
			fmt.Sprintf("{{%s}}", k), encoded,
			fmt.Sprintf("{%s}", k), encoded,
			fmt.Sprintf("{{input.%s}}", k), encoded,
		)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
