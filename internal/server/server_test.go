package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/config"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/server"
)

// newTestServer creates a server with a nil pool for handler-level tests.
func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	cfg := &config.Config{
		ServerPort:        "8083",
		KeycloakBaseURL:   "http://keycloak:8080",
		CORSAllowedOrigins: "*",
		LogLevel:          "info",
	}
	// Pass nil pool — only tests that don't hit /ready will use this.
	return server.New(cfg, nil)
}

func TestHealth_ReturnsUP(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"UP"`)
}

func TestSkillExecute_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/skills/my-skill/execute", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	// No Authorization header → 401 from Auth middleware.
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestToolExecute_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/tools/some-id/execute", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
