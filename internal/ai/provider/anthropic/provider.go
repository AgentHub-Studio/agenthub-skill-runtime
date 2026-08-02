// Package anthropic provides a ChatModel implementation backed by the Anthropic Messages API.
package anthropic

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

const (
	defaultBaseURL       = "https://api.anthropic.com/v1"
	anthropicVersion     = "2023-06-01"
	anthropicContentType = "application/json"
)

// Provider implements ai.ChatModel for the Anthropic Messages API.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New creates a new Anthropic Provider.
// If baseURL is empty, the default Anthropic API URL is used.
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

// GetProviderName returns "anthropic".
func (p *Provider) GetProviderName() string { return "anthropic" }

// ---- internal request / response types ----

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
	Tools     []tool    `json:"tools,omitempty"`
	Stream    bool      `json:"stream,omitempty"`
}

// message represents an Anthropic message, supporting both text and tool-result content.
type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// textContent is a simple text block.
type textContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// toolUseContent is a tool_use block returned by the model.
type toolUseContent struct {
	Type  string          `json:"type"` // "tool_use"
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// toolResultContent is the tool result block sent back by the caller.
type toolResultContent struct {
	Type      string `json:"type"` // "tool_result"
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type messagesResponse struct {
	ID         string         `json:"id"`
	Model      string         `json:"model"`
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"` // end_turn, tool_use, max_tokens
	Usage      anthropicUsage `json:"usage"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ---- stream event types ----

type streamEventEnvelope struct {
	Type  string          `json:"type"`
	Index int             `json:"index"`
	Delta *contentDelta   `json:"delta,omitempty"`
	Usage *anthropicUsage `json:"usage,omitempty"`
}

type contentDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// ---- ChatModel implementation ----

// Chat sends messages to the Anthropic Messages endpoint and returns a complete response.
func (p *Provider) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	req, err := p.buildRequest(messages, opts, false)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

	var msgResp messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	return p.convertResponse(&msgResp), nil
}

// ChatStream sends messages to the Anthropic Messages endpoint with streaming enabled.
// Token deltas are delivered via the returned channel; the channel is closed when the stream ends.
func (p *Provider) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	req, err := p.buildRequest(messages, opts, true)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create stream request: %w", err)
	}
	p.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: do stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, p.parseError(resp)
	}

	ch := make(chan ai.StreamChunk, 32)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// SSE lines are "event: ..." or "data: ..."
			data, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}

			var env streamEventEnvelope
			if err := json.Unmarshal([]byte(data), &env); err != nil {
				ch <- ai.StreamChunk{Error: fmt.Errorf("anthropic: decode stream event: %w", err)}
				return
			}

			switch env.Type {
			case "content_block_delta":
				if env.Delta == nil {
					continue
				}
				chunk := ai.StreamChunk{}
				switch env.Delta.Type {
				case "text_delta":
					chunk.Delta = env.Delta.Text
				case "input_json_delta":
					// Partial JSON for tool call arguments.
					chunk.ToolCallDelta = &ai.ToolCall{
						Function: ai.ToolFunction{Arguments: env.Delta.PartialJSON},
					}
				}
				ch <- chunk

			case "message_delta":
				if env.Delta != nil && env.Delta.StopReason != "" {
					ch <- ai.StreamChunk{FinishReason: mapStopReason(env.Delta.StopReason)}
				}

			case "message_stop":
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- ai.StreamChunk{Error: fmt.Errorf("anthropic: read stream: %w", err)}
		}
	}()

	return ch, nil
}

// ---- helpers ----

func (p *Provider) buildRequest(messages []ai.Message, opts ai.ChatOptions, stream bool) (*messagesRequest, error) {
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Anthropic requires max_tokens
	}

	req := &messagesRequest{
		Model:     opts.Model,
		MaxTokens: maxTokens,
		System:    opts.SystemMsg,
		Stream:    stream,
	}

	for _, m := range messages {
		// Skip system messages — they go in the top-level system field.
		if m.Role == ai.RoleSystem {
			if req.System == "" {
				req.System = m.Content
			}
			continue
		}

		msg, err := convertMessage(m)
		if err != nil {
			return nil, err
		}
		req.Messages = append(req.Messages, msg)
	}

	for _, t := range opts.Tools {
		req.Tools = append(req.Tools, tool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	return req, nil
}

func convertMessage(m ai.Message) (message, error) {
	switch {
	case m.Role == ai.RoleTool:
		// Tool result message.
		content := toolResultContent{
			Type:      "tool_result",
			ToolUseID: m.ToolCallID,
			Content:   m.Content,
		}
		raw, err := json.Marshal([]toolResultContent{content})
		if err != nil {
			return message{}, fmt.Errorf("anthropic: marshal tool result: %w", err)
		}
		return message{Role: "user", Content: raw}, nil

	case len(m.ToolCalls) > 0:
		// Assistant message with tool calls.
		var blocks []any
		if m.Content != "" {
			blocks = append(blocks, textContent{Type: "text", Text: m.Content})
		}
		for _, tc := range m.ToolCalls {
			inputRaw := json.RawMessage(tc.Function.Arguments)
			if !json.Valid(inputRaw) {
				inputRaw = json.RawMessage(`{}`)
			}
			blocks = append(blocks, toolUseContent{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputRaw,
			})
		}
		raw, err := json.Marshal(blocks)
		if err != nil {
			return message{}, fmt.Errorf("anthropic: marshal assistant tool_use: %w", err)
		}
		return message{Role: "assistant", Content: raw}, nil

	default:
		// Plain text message.
		raw, err := json.Marshal([]textContent{{Type: "text", Text: m.Content}})
		if err != nil {
			return message{}, fmt.Errorf("anthropic: marshal text content: %w", err)
		}
		return message{Role: m.Role, Content: raw}, nil
	}
}

func (p *Provider) convertResponse(r *messagesResponse) *ai.ChatResponse {
	resp := &ai.ChatResponse{
		Model:        r.Model,
		FinishReason: mapStopReason(r.StopReason),
		Usage: ai.Usage{
			PromptTokens:     r.Usage.InputTokens,
			CompletionTokens: r.Usage.OutputTokens,
			TotalTokens:      r.Usage.InputTokens + r.Usage.OutputTokens,
		},
	}

	for _, block := range r.Content {
		switch block.Type {
		case "text":
			resp.Content += block.Text
		case "tool_use":
			argsStr := "{}"
			if block.Input != nil {
				argsStr = string(block.Input)
			}
			resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ai.ToolFunction{
					Name:      block.Name,
					Arguments: argsStr,
				},
			})
		}
	}

	return resp
}

// mapStopReason converts Anthropic stop reasons to the common format.
func mapStopReason(r string) string {
	switch r {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return r
	}
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", anthropicContentType)
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
}

type apiError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *Provider) parseError(resp *http.Response) error {
	var apiErr apiError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("anthropic: API error %d: %s", resp.StatusCode, apiErr.Error.Message)
	}
	return fmt.Errorf("anthropic: unexpected status %d", resp.StatusCode)
}
