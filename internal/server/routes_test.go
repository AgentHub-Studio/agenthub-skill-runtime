package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseExecuteBody_FlatMapDirectToolCall(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/tools/tool-id/execute", strings.NewReader(`{"city":"Recife","limit":3}`))

	input, err := parseExecuteBody(req)
	if err != nil {
		t.Fatalf("parse execute body: %v", err)
	}

	if got := input["city"]; got != "Recife" {
		t.Fatalf("flat direct tool body city = %v, want Recife; input=%#v", got, input)
	}
	if got := input["limit"]; got != float64(3) {
		t.Fatalf("flat direct tool body limit = %v, want 3; input=%#v", got, input)
	}
}

func TestParseExecuteBody_FlatMapDirectToolCallPreservesInputNamedArgument(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/tools/tool-id/execute", strings.NewReader(`{"input":"Recife","format":"brief"}`))

	input, err := parseExecuteBody(req)
	if err != nil {
		t.Fatalf("parse execute body: %v", err)
	}

	if got := input["input"]; got != "Recife" {
		t.Fatalf("flat direct tool body input = %v, want Recife; input=%#v", got, input)
	}
	if got := input["format"]; got != "brief" {
		t.Fatalf("flat direct tool body format = %v, want brief; input=%#v", got, input)
	}
}

func TestParseExecuteBody_FlatMapDirectToolCallPreservesInputObjectWithSiblings(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/tools/tool-id/execute", strings.NewReader(`{"input":{"query":"Recife"},"format":"brief"}`))

	input, err := parseExecuteBody(req)
	if err != nil {
		t.Fatalf("parse execute body: %v", err)
	}

	nested, ok := input["input"].(map[string]any)
	if !ok {
		t.Fatalf("flat direct tool body input = %#v, want object; input=%#v", input["input"], input)
	}
	if got := nested["query"]; got != "Recife" {
		t.Fatalf("flat direct tool body input.query = %v, want Recife; input=%#v", got, input)
	}
	if got := input["format"]; got != "brief" {
		t.Fatalf("flat direct tool body format = %v, want brief; input=%#v", got, input)
	}
}

func TestParseExecuteBody_FlatMapDirectToolCallPreservesInputOnlyObjectArgument(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/tools/tool-id/execute", strings.NewReader(`{"input":{"query":"Recife"}}`))

	input, err := parseExecuteBody(req)
	if err != nil {
		t.Fatalf("parse execute body: %v", err)
	}

	nested, ok := input["input"].(map[string]any)
	if !ok {
		t.Fatalf("flat direct tool body input = %#v, want object; input=%#v", input["input"], input)
	}
	if got := nested["query"]; got != "Recife" {
		t.Fatalf("flat direct tool body input.query = %v, want Recife; input=%#v", got, input)
	}
}

func TestParseExecuteBody_FlatMapDirectToolCallPreservesContextNamedArgument(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/tools/tool-id/execute", strings.NewReader(`{"input":"Recife","context":"weather"}`))

	input, err := parseExecuteBody(req)
	if err != nil {
		t.Fatalf("parse execute body: %v", err)
	}

	if got := input["input"]; got != "Recife" {
		t.Fatalf("flat direct tool body input = %v, want Recife; input=%#v", got, input)
	}
	if got := input["context"]; got != "weather" {
		t.Fatalf("flat direct tool body context = %v, want weather; input=%#v", got, input)
	}
}

func TestParseExecuteBody_DirectToolEndpointPreservesRunnerContextShapeAsArgument(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/tools/tool-id/execute", strings.NewReader(`{
		"input":{"query":"Recife"},
		"context":{"tenantId":"tenant-as-tool-argument","scope":"weather"}
	}`))

	input, err := parseExecuteBody(req)
	if err != nil {
		t.Fatalf("parse execute body: %v", err)
	}

	nested, ok := input["input"].(map[string]any)
	if !ok {
		t.Fatalf("flat direct tool body input = %#v, want object; input=%#v", input["input"], input)
	}
	if got := nested["query"]; got != "Recife" {
		t.Fatalf("flat direct tool body input.query = %v, want Recife; input=%#v", got, input)
	}
	contextArg, ok := input["context"].(map[string]any)
	if !ok {
		t.Fatalf("flat direct tool body context = %#v, want object; input=%#v", input["context"], input)
	}
	if got := contextArg["tenantId"]; got != "tenant-as-tool-argument" {
		t.Fatalf("flat direct tool body context.tenantId = %v, want tenant-as-tool-argument; input=%#v", got, input)
	}
	if got := contextArg["scope"]; got != "weather" {
		t.Fatalf("flat direct tool body context.scope = %v, want weather; input=%#v", got, input)
	}
}

func TestParseExecuteBody_StructuredRunnerEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/skills/weather/execute", strings.NewReader(`{
		"input":{"city":"Recife","limit":3},
		"context":{"tenantId":"test","agentId":"agent-1","sessionId":"session-1"}
	}`))

	input, err := parseExecuteBody(req)
	if err != nil {
		t.Fatalf("parse execute body: %v", err)
	}

	if got := input["city"]; got != "Recife" {
		t.Fatalf("structured runner body city = %v, want Recife; input=%#v", got, input)
	}
	if _, exists := input["context"]; exists {
		t.Fatalf("structured runner context leaked into input: %#v", input)
	}
}

func TestParseExecuteBody_RejectsNonObjectInputEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/skills/weather/execute", strings.NewReader(`{
		"input":"Recife",
		"context":{"tenantId":"test","agentId":"agent-1","sessionId":"session-1"}
	}`))

	_, err := parseExecuteBody(req)
	if err == nil {
		t.Fatal("parse execute body accepted non-object input envelope")
	}
}
