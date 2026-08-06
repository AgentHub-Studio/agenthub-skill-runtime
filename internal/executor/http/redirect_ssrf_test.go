package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	httpexec "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/http"
)

func TestHTTPExecutor_BlocksRedirectToMetadataFromTrustedBackend(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	toolExecutor := httpexec.NewHTTPToolExecutor(server.URL)
	result, err := toolExecutor.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"url":    "/redirect",
			"method": "GET",
		},
	})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "redirect blocked")
	assert.Equal(t, 1, calls)
}
