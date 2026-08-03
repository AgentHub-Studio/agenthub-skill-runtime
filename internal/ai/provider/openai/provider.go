// Package openai provides a ChatModel implementation backed by the OpenAI Chat Completions API.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/ai"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Provider implements ai.ChatModel for the OpenAI API.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New creates a new OpenAI Provider.
// If baseURL is empty, the default OpenAI API URL is used.
func New(apiKey, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// GetProviderName returns "openai".
func (p *Provider) GetProviderName() string { return "openai" }

// ---- internal request / response types ----

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Tools       []tool    `json:"tools,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type tool struct {
	Type     string     `json:"type"`
	Function toolSchema `json:"function"`
}

type toolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

type choice struct {
	Index        int     `json:"index"`
	Message      message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// streamChoice represents one choice in a streaming SSE delta event.
type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type streamDelta struct {
	Role      string     `json:"role"`
	Content   *string    `json:"content"`
	ToolCalls []toolCall `json:"tool_calls"`
}

type streamEvent struct {
	Choices []streamChoice `json:"choices"`
}

// ---- ChatModel implementation ----

// Chat sends messages to the OpenAI Chat Completions endpoint and returns a complete response.
func (p *Provider) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	req := p.buildRequest(messages, opts, false)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	return p.convertResponse(&chatResp), nil
}

// ChatStream sends messages to the OpenAI Chat Completions endpoint with streaming enabled.
// Tokens are delivered via the returned channel; the channel is closed when the stream ends.
func (p *Provider) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	req := p.buildRequest(messages, opts, true)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create stream request: %w", err)
	}
	p.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: do stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		err := p.parseError(resp)
		_ = resp.Body.Close()
		return nil, err
	}

	ch := make(chan ai.StreamChunk, 32)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			if line == "" || line == "data: [DONE]" {
				if line == "data: [DONE]" {
					return
				}
				continue
			}

			data, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}

			var event streamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				ch <- ai.StreamChunk{Error: fmt.Errorf("openai: decode stream event: %w", err)}
				return
			}

			for _, choice := range event.Choices {
				chunk := ai.StreamChunk{}

				if choice.Delta.Content != nil {
					chunk.Delta = *choice.Delta.Content
				}

				if len(choice.Delta.ToolCalls) > 0 {
					tc := choice.Delta.ToolCalls[0]
					chunk.ToolCallDelta = &ai.ToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: ai.ToolFunction{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}

				if choice.FinishReason != nil {
					chunk.FinishReason = *choice.FinishReason
				}

				ch <- chunk
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- ai.StreamChunk{Error: fmt.Errorf("openai: read stream: %w", err)}
		}
	}()

	return ch, nil
}

// ---- helpers ----

func (p *Provider) buildRequest(messages []ai.Message, opts ai.ChatOptions, stream bool) chatRequest {
	msgs := make([]message, 0, len(messages)+1)

	// Prepend system message when provided and not already present.
	if opts.SystemMsg != "" {
		msgs = append(msgs, message{Role: ai.RoleSystem, Content: opts.SystemMsg})
	}

	for _, m := range messages {
		msg := message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, toolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: toolFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		msgs = append(msgs, msg)
	}

	tools := make([]tool, 0, len(opts.Tools))
	for _, t := range opts.Tools {
		tools = append(tools, tool{
			Type: t.Type,
			Function: toolSchema{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}

	return chatRequest{
		Model:       opts.Model,
		Messages:    msgs,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		TopP:        opts.TopP,
		Tools:       tools,
		Stream:      stream,
	}
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

func (p *Provider) convertResponse(r *chatResponse) *ai.ChatResponse {
	resp := &ai.ChatResponse{
		Model: r.Model,
		Usage: ai.Usage{
			PromptTokens:     r.Usage.PromptTokens,
			CompletionTokens: r.Usage.CompletionTokens,
			TotalTokens:      r.Usage.TotalTokens,
		},
	}

	if len(r.Choices) > 0 {
		c := r.Choices[0]
		resp.Content = c.Message.Content
		resp.FinishReason = c.FinishReason

		for _, tc := range c.Message.ToolCalls {
			resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ai.ToolFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return resp
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (p *Provider) parseError(resp *http.Response) error {
	var apiErr apiError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("openai: API error %d: %s", resp.StatusCode, apiErr.Error.Message)
	}
	return fmt.Errorf("openai: unexpected status %d", resp.StatusCode)
}
