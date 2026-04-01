package docsearch_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/docsearch"
)

func TestDocSearchExecutor_GetToolType(t *testing.T) {
	e := docsearch.NewDocumentSearchToolExecutor(nil)
	assert.Equal(t, "DOCUMENT_SEARCH", e.GetToolType())
}

func TestDocSearchExecutor_Execute_MissingKnowledgeBaseID(t *testing.T) {
	e := docsearch.NewDocumentSearchToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config: map[string]any{
			"embedding_url": "http://localhost:8080",
			// knowledge_base_id intentionally omitted
		},
		Input: map[string]any{"query": "hello"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knowledge_base_id is required")
}

func TestDocSearchExecutor_Execute_MissingEmbeddingURL(t *testing.T) {
	e := docsearch.NewDocumentSearchToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config: map[string]any{
			"knowledge_base_id": "some-uuid",
			// embedding_url intentionally omitted
		},
		Input: map[string]any{"query": "hello"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding_url is required")
}

func TestDocSearchExecutor_Execute_EmbeddingServiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	e := docsearch.NewDocumentSearchToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config: map[string]any{
			"knowledge_base_id": "some-uuid",
			"embedding_url":     srv.URL,
		},
		Input: map[string]any{"query": "find something"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embed query")
}

func TestDocSearchExecutor_Execute_EmbeddingServiceCalledCorrectly(t *testing.T) {
	// Verify that the embedding service is called with the correct path and method.
	// The executor calls the embedding service, then tries pgvector which panics on nil
	// pool, so we catch the panic and confirm the embedding request was made.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "/embed", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": []float32{0.1, 0.2, 0.3},
		})
	}))
	defer srv.Close()

	e := docsearch.NewDocumentSearchToolExecutor(nil)
	// The embedding call succeeds; the next step (pgvector with nil pool) panics.
	// Use assert.Panics to verify embedding was reached and the panic comes from the DB step.
	assert.Panics(t, func() {
		_, _ = e.Execute(context.Background(), executor.ExecutionContext{
			TenantID: "test-tenant",
			Config: map[string]any{
				"knowledge_base_id": "some-uuid",
				"embedding_url":     srv.URL,
			},
			Input: map[string]any{"query": "find something"},
		})
	})
	assert.True(t, called, "embedding service should have been called before the DB step")
}
