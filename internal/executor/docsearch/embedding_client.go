package docsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// EmbeddingClient calls the agenthub-embedding service to produce vector embeddings.
type EmbeddingClient struct {
	baseURL string
	client  *http.Client
}

// NewEmbeddingClient creates an EmbeddingClient targeting baseURL.
func NewEmbeddingClient(baseURL string) *EmbeddingClient {
	return &EmbeddingClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// embedRequest is the JSON body sent to the embedding service.
type embedRequest struct {
	Text string `json:"text"`
}

// embedResponse is the JSON body returned by the embedding service.
type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed sends text to the embedding service and returns the resulting vector.
func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(embedRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("embedding client: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding client: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding client: service returned %d: %s", resp.StatusCode, string(raw))
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("embedding client: decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("embedding client: empty embedding returned")
	}

	return result.Embedding, nil
}
