package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
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
//   - useCallerToken / use_caller_token: bool (forward the caller's JWT as Authorization: Bearer)
type HTTPToolExecutor struct {
	// backendBaseURL is prepended to relative URLs (starting with /).
	// URLs that begin with backendBaseURL are trusted (platform-configured) and
	// bypass the SSRF guard — the base URL itself is set by admin config, not by
	// user-supplied tool content.
	backendBaseURL string
	// urlValidator is the SSRF guard applied before every outbound request.
	// When nil, ValidateURL (the production SSRF guard) is used.
	// Override for testing via WithURLValidator.
	urlValidator func(string) error
	resolveHost  func(context.Context, string) ([]net.IPAddr, error)
	dialContext  func(context.Context, string, string) (net.Conn, error)
	// unsafeTestEgress is set only by WithURLValidator so existing component
	// tests can exercise local httptest servers. Production construction never
	// enables it and always uses the protected transport below.
	unsafeTestEgress bool
}

// NewHTTPToolExecutor creates an HTTPToolExecutor with the given backend base URL.
// The base URL is used for relative tool URLs (e.g. /api/skills → http://agenthub-api:8081/api/skills).
func NewHTTPToolExecutor(backendBaseURL string) *HTTPToolExecutor {
	dialer := &net.Dialer{}
	return &HTTPToolExecutor{
		backendBaseURL: backendBaseURL,
		resolveHost:    net.DefaultResolver.LookupIPAddr,
		dialContext:    dialer.DialContext,
	}
}

// WithURLValidator overrides the SSRF URL validator and protected transport.
// It exists only for component tests that use local httptest servers. Pass nil
// to restore the default production guards.
func (e *HTTPToolExecutor) WithURLValidator(fn func(string) error) *HTTPToolExecutor {
	e.urlValidator = fn
	e.unsafeTestEgress = fn != nil
	return e
}

// GetToolType returns the tool type identifier.
func (e *HTTPToolExecutor) GetToolType() string { return "HTTP" }

// httpConfig holds parsed configuration for an HTTP tool.
// BodyTemplate is normalised from three possible JSON field names:
// "bodyTemplate" (camelCase, preferred), "body_template" (snake_case, legacy),
// "body" (simple alias). See parseHTTPConfig for the resolution order (P-C160-1/P-C236-1).
// TimeoutSeconds, AuthType, AuthToken accept both camelCase and snake_case (BUG-TIMEOUT1 fix).
type httpConfig struct {
	URL            string            `json:"url"`
	URLTemplate    string            `json:"urlTemplate"` // alias used by some tool configs
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	BodyTemplate   string            // normalised — see parseHTTPConfig
	TimeoutSeconds int               // merged from timeoutSeconds (camelCase) and timeout_seconds (snake_case)
	AuthType       string            // merged from authType (camelCase) and auth_type (snake_case)
	AuthToken      string            // merged from authToken (camelCase) and auth_token (snake_case)
	UseCallerToken bool              // merged from useCallerToken (camelCase) and use_caller_token (snake_case)
	BaseURL        string            `json:"baseUrl"` // optional base URL prefix
	// AllowedInputKeys is the set of property names declared in the tool's
	// inputSchema.properties. When non-empty, appendUnusedInputAsQuery only
	// forwards keys that belong to this set — preventing LLM-hallucinated
	// args (e.g. "state":"SP") from being smuggled into the target URL and
	// rejected by strict upstreams like wttr.in (HTTP 500 ERR003).
	// Nil means "no schema declared", in which case all unused input keys
	// are forwarded (legacy BUG-HTTP-GET-PARAMS behaviour).
	AllowedInputKeys map[string]bool
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

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !isSupportedHTTPMethod(method) {
		return nil, fmt.Errorf("http executor: method must be one of GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS (got %q)", cfg.Method)
	}
	authType, err := normalizeHTTPAuthType(cfg.AuthType)
	if err != nil {
		return nil, fmt.Errorf("http executor: %w", err)
	}

	renderedURL := renderTemplateURL(rawURL, ec.Input)

	// BUG-HTTP-GET-PARAMS: for GET (and DELETE/HEAD) requests, append any input
	// parameters that were not consumed as URL template placeholders as query string
	// parameters. This ensures that when a tool's URL has no {city} placeholder
	// but the LLM provides city=Tokyo, the parameter is forwarded as ?city=Tokyo.
	if method == http.MethodGet || method == http.MethodDelete || method == http.MethodHead {
		renderedURL = appendUnusedInputAsQuery(rawURL, renderedURL, ec.Input, cfg.AllowedInputKeys)
	}

	if containsUnsafeURLPathToken(renderedURL) {
		return nil, fmt.Errorf("http executor: path traversal blocked in URL")
	}

	// P-C220-1 / P-C221-1: reject SSRF attempts before sending any request.
	// Exception: URLs rooted at backendBaseURL are platform-configured (admin-set)
	// and trusted — they are not user-supplied and bypass the SSRF guard so that
	// core tools (e.g. agenthub_list_skills) can call back to the API service.
	trustedBase := strings.TrimRight(e.backendBaseURL, "/")
	isTrustedBackend := trustedBase != "" && (strings.HasPrefix(renderedURL, trustedBase+"/") || renderedURL == trustedBase)
	if !isTrustedBackend {
		validate := e.urlValidator
		if validate == nil {
			validate = ValidateURL // production default
		}
		if err := validate(renderedURL); err != nil {
			return nil, fmt.Errorf("http executor: blocked URL (%w)", err)
		}
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
	case authType == "bearer" && cfg.AuthToken != "":
		req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	case authType == "basic" && cfg.AuthToken != "":
		req.Header.Set("Authorization", "Basic "+cfg.AuthToken)
	}

	client := e.newHTTPClient(trustedBase)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http executor: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http executor: read response body: %w", err)
	}

	// Attempt to parse response as JSON; fall back to raw string.
	var responsePayload any
	if err := json.Unmarshal(respBytes, &responsePayload); err != nil {
		responsePayload = string(respBytes)
	}

	// P-F1-1: Treat 4xx/5xx status codes as errors so the LLM receives a clear
	// failure signal instead of an empty/partial response that causes it to retry.
	if resp.StatusCode >= 400 {
		body := ""
		if s, ok := responsePayload.(string); ok {
			body = s
		} else if responsePayload != nil {
			if b, err := json.Marshal(responsePayload); err == nil {
				body = string(b)
			}
		}
		msg := fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		if body != "" {
			msg = fmt.Sprintf("%s: %s", msg, body)
		}
		return nil, fmt.Errorf("http executor: %s", msg)
	}

	output := map[string]any{
		"response":    responsePayload,
		"status_code": resp.StatusCode,
	}

	return &executor.Result{Output: output}, nil
}

func (e *HTTPToolExecutor) newHTTPClient(trustedBase string) *http.Client {
	if e.unsafeTestEgress {
		return &http.Client{}
	}

	trustedAddress := trustedBackendAddress(trustedBase)
	transport := &http.Transport{
		// Proxy environment variables would create an unvalidated egress path.
		// HTTP tools therefore dial the validated destination directly.
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return e.dialOutbound(ctx, network, address, trustedAddress)
		},
	}

	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("http executor: too many redirects")
			}
			if isTrustedBackendURL(req.URL.String(), trustedBase) {
				return nil
			}
			if err := ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("http executor: redirect blocked (%w)", err)
			}
			return nil
		},
	}
}

func isTrustedBackendURL(rawURL, trustedBase string) bool {
	trustedBase = strings.TrimRight(trustedBase, "/")
	return trustedBase != "" && (rawURL == trustedBase || strings.HasPrefix(rawURL, trustedBase+"/"))
}

func trustedBackendAddress(trustedBase string) string {
	if trustedBase == "" {
		return ""
	}
	u, err := url.Parse(trustedBase)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(normalizeHost(u.Hostname()), port)
}

// dialOutbound resolves each non-trusted host exactly once, rejects private or
// reserved targets, and dials the selected IP directly. This avoids DNS
// rebinding between ValidateURL and http.Transport's later connection attempt.
func (e *HTTPToolExecutor) dialOutbound(ctx context.Context, network, address, trustedAddress string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("http executor: invalid outbound address: %w", err)
	}
	if strings.EqualFold(net.JoinHostPort(normalizeHost(host), port), trustedAddress) {
		return e.dialContext(ctx, network, address)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedOutboundIP(ip) {
			return nil, fmt.Errorf("http executor: outbound target is blocked: %s", ip)
		}
		return e.dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	addresses, err := e.resolveHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("http executor: resolve outbound host %q: %w", host, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("http executor: outbound host %q resolved without addresses", host)
	}
	for _, resolved := range addresses {
		if isBlockedOutboundIP(resolved.IP) {
			return nil, fmt.Errorf("http executor: outbound host %q resolves to blocked address %s", host, resolved.IP)
		}
	}

	return e.dialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func isBlockedOutboundIP(ip net.IP) bool {
	return IsBlockedOutboundIP(ip)
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

func isSupportedHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func normalizeHTTPAuthType(authType string) (string, error) {
	if authType == "" {
		return "none", nil
	}
	if strings.TrimSpace(authType) != authType {
		return "", fmt.Errorf("authType must be one of none, bearer, basic (got %q)", authType)
	}
	authType = strings.ToLower(authType)
	switch authType {
	case "none", "bearer", "basic":
		return authType, nil
	default:
		return "", fmt.Errorf("authType must be one of none, bearer, basic (got %q)", authType)
	}
}

func containsUnsafeURLPathToken(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return containsUnsafePathToken(raw)
	}

	for _, candidate := range []string{u.EscapedPath(), u.RawPath, u.Path} {
		if containsUnsafePathToken(candidate) {
			return true
		}
	}
	return false
}

func containsUnsafePathToken(path string) bool {
	candidate := strings.TrimSpace(path)
	for i := 0; i < 3; i++ {
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, "../") ||
			strings.Contains(lower, `..\`) ||
			strings.Contains(lower, "%2f") ||
			strings.Contains(lower, "%5c") {
			return true
		}
		decoded, err := url.PathUnescape(candidate)
		if err != nil || decoded == candidate {
			break
		}
		candidate = decoded
	}
	return false
}

// parseHTTPConfig decodes the executor config map into an httpConfig struct.
// It resolves BodyTemplate from three possible field names with the following
// priority: "bodyTemplate" (camelCase) > "body_template" (snake_case) > "body".
// This preserves backwards compatibility with legacy configs while accepting
// the camelCase format used by the frontend and migrations (P-C160-1/P-C236-1).
func parseHTTPConfig(raw map[string]any) (*httpConfig, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	if err := validateHTTPConfigAliasContract(raw); err != nil {
		return nil, err
	}

	// rawHTTPConfig mirrors httpConfig but exposes all three body field aliases
	// so we can apply priority logic after unmarshaling.
	// Body template fields use json.RawMessage to accept both string and object
	// values — some core tool configs store bodyTemplate as a JSON object for
	// readability; we normalise them to a compact JSON string (P-KB2-1).
	type rawHTTPConfig struct {
		URL            string            `json:"url"`
		URLTemplate    string            `json:"urlTemplate"`
		Method         string            `json:"method"`
		Headers        map[string]string `json:"headers"`
		BodyTemplate   json.RawMessage   `json:"bodyTemplate"`  // camelCase (preferred)
		BodyTemplateSC json.RawMessage   `json:"body_template"` // snake_case (legacy)
		Body           json.RawMessage   `json:"body"`          // simple alias
		// BUG-TIMEOUT1: accept both camelCase (API convention) and snake_case (legacy).
		// Prefer camelCase; snake_case values are merged after unmarshaling.
		TimeoutSeconds   int    `json:"timeoutSeconds"`  // camelCase (preferred)
		TimeoutSecondsSC int    `json:"timeout_seconds"` // snake_case (legacy)
		TimeoutMs        int    `json:"timeoutMs"`       // milliseconds (frontend/API convention)
		AuthType         string `json:"authType"`        // camelCase (preferred)
		AuthTypeSC       string `json:"auth_type"`       // snake_case (legacy)
		AuthToken        string `json:"authToken"`       // camelCase (preferred)
		AuthTokenSC      string `json:"auth_token"`      // snake_case (legacy)
		UseCallerToken   *bool  `json:"useCallerToken"`
		UseCallerTokenSC *bool  `json:"use_caller_token"`
		BaseURL          string `json:"baseUrl"`
		// InputSchema mirrors the JSON Schema that ships with every tool config.
		// We only care about the "properties" map here — its keys define which
		// LLM-supplied inputs are legitimate args. Keys NOT listed there are
		// dropped before being appended as query params (BUG-EXTRA-PARAMS).
		InputSchema json.RawMessage `json:"inputSchema"`
	}

	var r rawHTTPConfig
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// BUG-TIMEOUT1: merge camelCase (preferred) and snake_case (legacy) variants.
	// camelCase wins when both are present and non-zero.
	// Also support timeoutMs (milliseconds) as used by the frontend/API — convert to seconds.
	timeoutSeconds := r.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = r.TimeoutSecondsSC
	}
	if timeoutSeconds == 0 && r.TimeoutMs > 0 {
		// Round up to nearest second; ensure at least 1s for small values.
		timeoutSeconds = (r.TimeoutMs + 999) / 1000
		if timeoutSeconds < 1 {
			timeoutSeconds = 1
		}
	}
	authType := r.AuthType
	if authType == "" {
		authType = r.AuthTypeSC
	}
	authToken := r.AuthToken
	if authToken == "" {
		authToken = r.AuthTokenSC
	}
	useCallerToken := false
	if r.UseCallerToken != nil {
		useCallerToken = *r.UseCallerToken
	} else if r.UseCallerTokenSC != nil {
		useCallerToken = *r.UseCallerTokenSC
	}

	cfg := &httpConfig{
		URL:              r.URL,
		URLTemplate:      r.URLTemplate,
		Method:           r.Method,
		Headers:          r.Headers,
		TimeoutSeconds:   timeoutSeconds,
		AuthType:         authType,
		AuthToken:        authToken,
		UseCallerToken:   useCallerToken,
		BaseURL:          r.BaseURL,
		AllowedInputKeys: extractSchemaKeys(r.InputSchema),
	}

	// Priority: bodyTemplate > body_template > body.
	// normaliseBody converts a json.RawMessage to a string:
	//   - JSON string → unquoted value (e.g. `"foo"` → `foo`)
	//   - JSON object/array → compact JSON string (e.g. `{"k":"v"}` → `{"k":"v"}`)
	//   - null / empty → empty string
	normaliseBody := func(raw json.RawMessage) string {
		if len(raw) == 0 || string(raw) == "null" {
			return ""
		}
		if raw[0] == '"' {
			// Already a JSON string — unquote it.
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				return s
			}
		}
		// Object or array — compact it into a JSON string template.
		var buf bytes.Buffer
		if err := json.Compact(&buf, raw); err == nil {
			return buf.String()
		}
		return string(raw)
	}

	switch {
	case len(r.BodyTemplate) > 0 && string(r.BodyTemplate) != "null":
		cfg.BodyTemplate = normaliseBody(r.BodyTemplate)
	case len(r.BodyTemplateSC) > 0 && string(r.BodyTemplateSC) != "null":
		cfg.BodyTemplate = normaliseBody(r.BodyTemplateSC)
	case len(r.Body) > 0 && string(r.Body) != "null":
		cfg.BodyTemplate = normaliseBody(r.Body)
	}

	return cfg, nil
}

// validateHTTPConfigAliasContract keeps legacy HTTP aliases only when they
// produce the same effective runtime value. This prevents old persisted
// configs from making outbound requests with order-dependent semantics.
func validateHTTPConfigAliasContract(raw map[string]any) error {
	if err := validateHTTPStringAliases(raw, "url", "urlTemplate", func(value string) string { return value }); err != nil {
		return err
	}
	if err := validateHTTPBodyAliases(raw); err != nil {
		return err
	}
	if err := validateHTTPTimeoutAliases(raw); err != nil {
		return err
	}
	if err := validateHTTPStringAliases(raw, "authType", "auth_type", strings.ToLower); err != nil {
		return err
	}
	if err := validateHTTPStringAliases(raw, "authToken", "auth_token", func(value string) string { return value }); err != nil {
		return err
	}
	if err := validateHTTPBoolAliases(raw, "useCallerToken", "use_caller_token"); err != nil {
		return err
	}
	return nil
}

func validateHTTPBodyAliases(raw map[string]any) error {
	canonical := ""
	canonicalField := ""
	for _, key := range []string{"bodyTemplate", "body_template", "body"} {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		valueCanonical, err := canonicalHTTPBodyTemplate(value)
		if err != nil {
			return err
		}
		if canonicalField != "" && canonical != valueCanonical {
			return conflictingHTTPAliasError(canonicalField, key)
		}
		canonical = valueCanonical
		canonicalField = key
	}
	return nil
}

func canonicalHTTPBodyTemplate(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal HTTP body alias: %w", err)
	}
	if len(raw) > 0 && raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			raw = []byte(text)
		}
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw), nil
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return string(raw), nil
	}
	return string(canonical), nil
}

func validateHTTPTimeoutAliases(raw map[string]any) error {
	canonical := 0
	canonicalField := ""
	for _, field := range []struct {
		key      string
		toSecond func(int) int
	}{
		{key: "timeoutSeconds", toSecond: func(value int) int { return value }},
		{key: "timeout_seconds", toSecond: func(value int) int { return value }},
		{key: "timeoutMs", toSecond: func(value int) int { return (value + 999) / 1000 }},
	} {
		value, ok := positiveHTTPAliasInt(raw[field.key])
		if !ok {
			continue
		}
		value = field.toSecond(value)
		if canonicalField != "" && canonical != value {
			return conflictingHTTPAliasError(canonicalField, field.key)
		}
		canonical = value
		canonicalField = field.key
	}
	return nil
}

func positiveHTTPAliasInt(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, value > 0
	case int64:
		return int(value), value > 0
	case float64:
		if math.Trunc(value) != value || value <= 0 {
			return 0, false
		}
		return int(value), true
	case json.Number:
		integer, err := value.Int64()
		return int(integer), err == nil && integer > 0
	default:
		return 0, false
	}
}

func validateHTTPStringAliases(raw map[string]any, camelKey, snakeKey string, normalize func(string) string) error {
	camel, camelSet := raw[camelKey].(string)
	snake, snakeSet := raw[snakeKey].(string)
	if !camelSet || camel == "" || !snakeSet || snake == "" {
		return nil
	}
	if normalize(camel) != normalize(snake) {
		return conflictingHTTPAliasError(camelKey, snakeKey)
	}
	return nil
}

func validateHTTPBoolAliases(raw map[string]any, camelKey, snakeKey string) error {
	camel, camelSet := raw[camelKey].(bool)
	snake, snakeSet := raw[snakeKey].(bool)
	if !camelSet || !snakeSet || camel == snake {
		return nil
	}
	return conflictingHTTPAliasError(camelKey, snakeKey)
}

func conflictingHTTPAliasError(first, second string) error {
	return fmt.Errorf("conflicting HTTP config aliases %s and %s", first, second)
}

// renderTemplate replaces placeholders in tmpl with corresponding string values
// from input. Supports four formats (matched in priority order to avoid
// partial substitution of double-brace templates):
//   - {{key}}         — Handlebars/Mustache-style (most intuitive for users)
//   - {key}           — single-brace format stored in the database
//   - {{input.key}}   — explicit input-namespace format
//   - {{args.key}}    — LLM tool-call args namespace (bug 197); matches the
//     mental model when authoring tool configs from agent perspective
//     (LLM tool_call.arguments are presented as `args` to the runtime).
func renderTemplate(tmpl string, input map[string]any) string {
	pairs := make([]string, 0, len(input)*8)
	for k, v := range input {
		val := fmt.Sprintf("%v", v)
		// {{key}} must come BEFORE {key} so that double-brace templates are
		// matched first; otherwise {key} inside {{key}} would be replaced first,
		// producing {value} (with stray curly braces) instead of value.
		pairs = append(pairs,
			fmt.Sprintf("{{%s}}", k), val,
			fmt.Sprintf("{%s}", k), val,
			fmt.Sprintf("{{input.%s}}", k), val,
			fmt.Sprintf("{{args.%s}}", k), val,
		)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

// appendUnusedInputAsQuery appends input keys that were NOT consumed as {key}
// template placeholders in the original URL template as query string parameters
// to the already-rendered URL.
//
// BUG-HTTP-GET-PARAMS: for GET requests without URL template vars, parameters
// provided by the LLM were silently dropped. This fix forwards them as query params.
//
// BUG-EXTRA-PARAMS: when allowedKeys is non-nil, only keys listed in the tool's
// inputSchema.properties are forwarded. LLMs routinely invent extra fields
// (e.g. "state":"SP" for a weather tool that only declares "city" and "units")
// and upstreams that reject unknown query params (wttr.in → HTTP 500 ERR003)
// would break every such call. When allowedKeys is nil the schema is unknown
// and legacy behaviour (forward all) is preserved.
func appendUnusedInputAsQuery(rawTemplate, renderedURL string, input map[string]any, allowedKeys map[string]bool) string {
	if len(input) == 0 {
		return renderedURL
	}
	// Determine which keys were used as template placeholders in the original URL.
	used := map[string]bool{}
	for k := range input {
		if strings.Contains(rawTemplate, "{"+k+"}") ||
			strings.Contains(rawTemplate, "{{"+k+"}}") ||
			strings.Contains(rawTemplate, "{{input."+k+"}}") ||
			strings.Contains(rawTemplate, "{{args."+k+"}}") {
			used[k] = true
		}
	}
	// Build query params for keys that were not substituted. When the tool
	// declared an inputSchema, also drop keys that aren't in its properties.
	qv := url.Values{}
	for k, v := range input {
		if used[k] {
			continue
		}
		if allowedKeys != nil && !allowedKeys[k] {
			continue
		}
		qv.Set(k, fmt.Sprintf("%v", v))
	}
	if len(qv) == 0 {
		return renderedURL
	}
	sep := "?"
	if strings.Contains(renderedURL, "?") {
		sep = "&"
	}
	return renderedURL + sep + qv.Encode()
}

// extractSchemaKeys returns the set of property names declared in a JSON
// Schema payload's "properties" object, or nil when the payload is empty,
// malformed, or lacks a properties map. Only top-level property names are
// considered — nested schemas are intentionally ignored because they apply
// to nested values that are never forwarded as query params.
func extractSchemaKeys(raw json.RawMessage) map[string]bool {
	trimmed := string(raw)
	if len(raw) == 0 || trimmed == "null" {
		return nil
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}
	if len(schema.Properties) == 0 {
		return nil
	}
	keys := make(map[string]bool, len(schema.Properties))
	for k := range schema.Properties {
		keys[k] = true
	}
	return keys
}

// renderTemplateURL is like renderTemplate but URL-encodes each substituted value.
// P-C285-1: prevents malformed URLs when input values contain spaces, &, =, +, etc.
// Should be used only for URL rendering, NOT for body/header templates.
// Bug 197: also supports {{args.key}} namespace (LLM tool-call args).
func renderTemplateURL(tmpl string, input map[string]any) string {
	pairs := make([]string, 0, len(input)*8)
	for k, v := range input {
		encoded := url.QueryEscape(fmt.Sprintf("%v", v))
		pairs = append(pairs,
			fmt.Sprintf("{{%s}}", k), encoded,
			fmt.Sprintf("{%s}", k), encoded,
			fmt.Sprintf("{{input.%s}}", k), encoded,
			fmt.Sprintf("{{args.%s}}", k), encoded,
		)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
