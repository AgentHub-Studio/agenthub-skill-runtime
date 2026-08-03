package http

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestAppendUnusedInputAsQuery_FiltersBySchema(t *testing.T) {
	// Weather-style scenario: schema declares "city" and "units"; the LLM
	// hallucinates an extra "state":"SP" arg. Before the fix it was forwarded
	// as ?state=SP and wttr.in responded HTTP 500 ERR003. After the fix it
	// is dropped because "state" isn't in allowedKeys.
	rawURL := "https://wttr.in/{city}?format=3"
	rendered := "https://wttr.in/S%C3%A3oPaulo?format=3"
	input := map[string]any{
		"city":  "SãoPaulo",
		"units": "celsius",
		"state": "SP", // hallucinated
	}
	allowed := map[string]bool{"city": true, "units": true}

	got := appendUnusedInputAsQuery(rawURL, rendered, input, allowed)
	if hasQueryParam(t, got, "state") {
		t.Errorf("expected 'state' param to be dropped; got %q", got)
	}
	if !hasQueryParam(t, got, "units") {
		t.Errorf("expected 'units' to be forwarded; got %q", got)
	}
}

func TestAppendUnusedInputAsQuery_NilSchemaForwardsAll(t *testing.T) {
	// Backward compat: tools with no inputSchema keep the legacy behaviour —
	// every unused key is forwarded.
	rawURL := "https://api.example.com/search"
	input := map[string]any{"q": "hi", "limit": 10}

	got := appendUnusedInputAsQuery(rawURL, rawURL, input, nil)
	if !hasQueryParam(t, got, "q") || !hasQueryParam(t, got, "limit") {
		t.Errorf("nil schema must forward all keys; got %q", got)
	}
}

func TestAppendUnusedInputAsQuery_TemplateUsedKeysNotForwarded(t *testing.T) {
	// Keys already substituted into the path must not also appear as query.
	rawURL := "https://api.example.com/users/{id}"
	rendered := "https://api.example.com/users/42"
	input := map[string]any{"id": "42", "verbose": "true"}
	allowed := map[string]bool{"id": true, "verbose": true}

	got := appendUnusedInputAsQuery(rawURL, rendered, input, allowed)
	if hasQueryParam(t, got, "id") {
		t.Errorf("path-substituted key must not be appended; got %q", got)
	}
	if !hasQueryParam(t, got, "verbose") {
		t.Errorf("unused allowed key must be appended; got %q", got)
	}
}

func TestAppendUnusedInputAsQuery_ArgsNamespaceTemplateKeyNotForwarded(t *testing.T) {
	rawURL := "https://api.example.com/users/{{args.id}}"
	rendered := "https://api.example.com/users/42"
	input := map[string]any{"id": "42", "verbose": "true"}
	allowed := map[string]bool{"id": true, "verbose": true}

	got := appendUnusedInputAsQuery(rawURL, rendered, input, allowed)
	if hasQueryParam(t, got, "id") {
		t.Errorf("args namespace path-substituted key must not be appended; got %q", got)
	}
	if !hasQueryParam(t, got, "verbose") {
		t.Errorf("unused allowed key must be appended; got %q", got)
	}
}

func TestAppendUnusedInputAsQuery_EmptyInput(t *testing.T) {
	got := appendUnusedInputAsQuery("x", "x", map[string]any{}, nil)
	if got != "x" {
		t.Errorf("empty input must not mutate URL; got %q", got)
	}
}

func TestExtractSchemaKeys_Properties(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"},"units":{"type":"string"}},"required":["city"]}`)
	got := extractSchemaKeys(raw)
	if !got["city"] || !got["units"] {
		t.Errorf("expected city+units, got %v", got)
	}
	if got["required"] {
		t.Errorf("must not pick up top-level non-properties keys")
	}
}

func FuzzAppendUnusedInputAsQueryDoesNotForwardTemplateKeys(f *testing.F) {
	f.Add("id", "42", "{{args.%s}}")
	f.Add("city", "São Paulo", "{{input.%s}}")
	f.Add("q", "hello world", "{{%s}}")
	f.Add("user", "alice@example.com", "{%s}")

	for _, seed := range []string{"id", "city", "q", "user"} {
		f.Add(seed, "value", "{{args.%s}}")
	}

	f.Fuzz(func(t *testing.T, key, value, placeholderPattern string) {
		key = safeFuzzKey(key)
		if key == "" {
			t.Skip()
		}
		switch placeholderPattern {
		case "{{args.%s}}", "{{input.%s}}", "{{%s}}", "{%s}":
		default:
			placeholderPattern = "{{args.%s}}"
		}

		rawURL := "https://api.example.test/resource/" + strings.ReplaceAll(placeholderPattern, "%s", key)
		rendered := "https://api.example.test/resource/rendered"
		input := map[string]any{key: value, "unused": "kept"}
		allowed := map[string]bool{key: true, "unused": true}

		got := appendUnusedInputAsQuery(rawURL, rendered, input, allowed)
		if hasQueryParam(t, got, key) {
			t.Fatalf("template key %q was forwarded as query param: %q", key, got)
		}
		if !hasQueryParam(t, got, "unused") {
			t.Fatalf("unused allowed key was not forwarded: %q", got)
		}
	})
}

func hasQueryParam(t *testing.T, rawURL, key string) bool {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL %q: %v", rawURL, err)
	}
	_, ok := u.Query()[key]
	return ok
}

func safeFuzzKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
		if b.Len() >= 24 {
			break
		}
	}
	return b.String()
}

func TestExtractSchemaKeys_EdgeCases(t *testing.T) {
	cases := []json.RawMessage{
		nil,
		json.RawMessage(``),
		json.RawMessage(`null`),
		json.RawMessage(`{}`),
		json.RawMessage(`{"type":"object"}`), // no properties
		json.RawMessage(`{"properties":{}}`), // empty properties
		json.RawMessage(`{"properties":"not-an-object"}`), // malformed
		json.RawMessage(`not-json`),
	}
	for _, c := range cases {
		if got := extractSchemaKeys(c); got != nil {
			t.Errorf("expected nil for %q, got %v", string(c), got)
		}
	}
}
