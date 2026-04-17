package http

import (
	"encoding/json"
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
	if strings.Contains(got, "state=") {
		t.Errorf("expected 'state' param to be dropped; got %q", got)
	}
	if !strings.Contains(got, "units=celsius") {
		t.Errorf("expected 'units' to be forwarded; got %q", got)
	}
}

func TestAppendUnusedInputAsQuery_NilSchemaForwardsAll(t *testing.T) {
	// Backward compat: tools with no inputSchema keep the legacy behaviour —
	// every unused key is forwarded.
	rawURL := "https://api.example.com/search"
	input := map[string]any{"q": "hi", "limit": 10}

	got := appendUnusedInputAsQuery(rawURL, rawURL, input, nil)
	if !strings.Contains(got, "q=hi") || !strings.Contains(got, "limit=10") {
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
	if strings.Contains(got, "id=") {
		t.Errorf("path-substituted key must not be appended; got %q", got)
	}
	if !strings.Contains(got, "verbose=true") {
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

func TestExtractSchemaKeys_EdgeCases(t *testing.T) {
	cases := []json.RawMessage{
		nil,
		json.RawMessage(``),
		json.RawMessage(`null`),
		json.RawMessage(`{}`),
		json.RawMessage(`{"type":"object"}`),          // no properties
		json.RawMessage(`{"properties":{}}`),            // empty properties
		json.RawMessage(`{"properties":"not-an-object"}`), // malformed
		json.RawMessage(`not-json`),
	}
	for _, c := range cases {
		if got := extractSchemaKeys(c); got != nil {
			t.Errorf("expected nil for %q, got %v", string(c), got)
		}
	}
}
