package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/middleware"
)

func TestAuth_MissingAuthorizationHeader(t *testing.T) {
	handler := middleware.Auth("http://keycloak:8080")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_InvalidBearerFormat(t *testing.T) {
	handler := middleware.Auth("http://keycloak:8080")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_MalformedToken(t *testing.T) {
	handler := middleware.Auth("http://keycloak:8080")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.jwt")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTenantIDFromContext_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	id := middleware.TenantIDFromContext(req.Context())
	assert.Empty(t, id)
}
