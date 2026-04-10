package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	httpexec "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/http"
)

// noSSRFExecutor returns an executor with SSRF validation disabled for tests that
// use a local httptest.Server (127.0.0.1). Production callers always use the
// default executor (which enforces ValidateURL).
func noSSRFExecutor() *httpexec.HTTPToolExecutor {
	return httpexec.NewHTTPToolExecutor("").WithURLValidator(func(string) error { return nil })
}

func TestHTTPExecutor_GetToolType(t *testing.T) {
	e := httpexec.NewHTTPToolExecutor("")
	assert.Equal(t, "HTTP", e.GetToolType())
}

func TestHTTPExecutor_SimpleGET_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"city": "São Paulo"})
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"url": srv.URL, "method": "GET"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Output["status_code"])
	body, ok := res.Output["response"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "São Paulo", body["city"])
}

func TestHTTPExecutor_POSTWithBodyTemplate(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":           srv.URL,
			"method":        "POST",
			"body_template": `{"name":"{{input.name}}"}`,
		},
		Input: map[string]any{"name": "Alice"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, res.Output["status_code"])
	assert.Equal(t, "Alice", received["name"])
}

func TestHTTPExecutor_URLTemplate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/42", r.URL.Path)
		_, _ = w.Write([]byte(`"ok"`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"url": srv.URL + "/users/{{input.id}}", "method": "GET"},
		Input:  map[string]any{"id": "42"},
	})
	require.NoError(t, err)
}

func TestHTTPExecutor_BearerAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":        srv.URL,
			"method":     "GET",
			"auth_type":  "bearer",
			"auth_token": "secret-token",
		},
	})
	require.NoError(t, err)
}

func TestHTTPExecutor_CustomHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":     srv.URL,
			"method":  "GET",
			"headers": map[string]any{"Accept": "application/json"},
		},
	})
	require.NoError(t, err)
}

func TestHTTPExecutor_NonJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain text response"))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"url": srv.URL},
	})
	require.NoError(t, err)
	assert.Equal(t, "plain text response", res.Output["response"])
}

func TestHTTPExecutor_MissingURL(t *testing.T) {
	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url is required")
}

func TestHTTPExecutor_InvalidURL(t *testing.T) {
	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"url": "://bad-url"},
	})
	require.Error(t, err)
}

// TestHTTPExecutor_URLTemplateQueryEncoding validates P-C285-1 fix:
// values substituted into the URL must be percent-encoded so that special
// characters (spaces, &, =, +) do not corrupt the request URL.
func TestHTTPExecutor_URLTemplateQueryEncoding(t *testing.T) {
	var receivedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the raw query string as parsed by the HTTP server.
		receivedQuery = r.URL.Query().Get("q")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":    srv.URL + "/search?q={{input.query}}",
			"method": "GET",
		},
		// P-C285-1: this value contains spaces and special chars that must be encoded.
		Input: map[string]any{"query": "hello world & more"},
	})
	require.NoError(t, err)
	// The server should receive the decoded value — net/http automatically decodes
	// the query string, so if encoding was correct the original value is recovered.
	assert.Equal(t, "hello world & more", receivedQuery)
}

// --- TR-01-TASK-08: bodyTemplate/body camelCase aliases (P-C160-1/P-C236-1) ---

// TestHTTPExecutor_CamelCase_BodyTemplate verifies that "bodyTemplate" (camelCase)
// is accepted and the body is rendered and sent.
func TestHTTPExecutor_CamelCase_BodyTemplate(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":          srv.URL,
			"method":       "POST",
			"bodyTemplate": `{"city":"{{city}}"}`, // camelCase key
		},
		Input: map[string]any{"city": "Curitiba"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Curitiba", received["city"])
}

// TestHTTPExecutor_Body_Alias verifies that "body" is accepted as a simple alias.
func TestHTTPExecutor_Body_Alias(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":    srv.URL,
			"method": "POST",
			"body":   `{"value":"{{val}}"}`, // simple "body" key
		},
		Input: map[string]any{"val": "test"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test", received["value"])
}

// TestHTTPExecutor_CamelCasePrecedesSnakeCase verifies that "bodyTemplate" takes
// priority over "body_template" when both are present.
func TestHTTPExecutor_CamelCasePrecedesSnakeCase(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	e := noSSRFExecutor()
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":           srv.URL,
			"method":        "POST",
			"bodyTemplate":  `{"source":"camel"}`, // camelCase wins
			"body_template": `{"source":"snake"}`,
		},
	})
	require.NoError(t, err)
	assert.Contains(t, string(receivedBody), "camel")
	assert.NotContains(t, string(receivedBody), "snake")
}
