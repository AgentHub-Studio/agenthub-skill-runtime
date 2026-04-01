package ai

import "context"

// Role constants for chat messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)

// Message represents a single chat message.
type Message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"toolCallId,omitempty"`
	ToolCalls  []ToolCall     `json:"toolCalls,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ToolCall represents an LLM tool call request.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction holds the name and arguments of a tool call.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Tool defines a tool available to the LLM.
type Tool struct {
	Type     string     `json:"type"` // "function"
	Function ToolSchema `json:"function"`
}

// ToolSchema describes a function tool.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// ChatOptions holds configuration for a chat request.
type ChatOptions struct {
	Model       string  `json:"model"`
	MaxTokens   int     `json:"maxTokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"topP,omitempty"`
	Tools       []Tool  `json:"tools,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
	SystemMsg   string  `json:"systemMessage,omitempty"`
}

// ChatResponse holds the response from a chat request.
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"toolCalls,omitempty"`
	FinishReason string     `json:"finishReason"` // stop, tool_calls, length
	Usage        Usage      `json:"usage"`
	Model        string     `json:"model"`
}

// Usage holds token usage statistics.
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// ChatModel is the core interface for LLM providers.
type ChatModel interface {
	// Chat sends a chat request and returns a complete response.
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error)

	// ChatStream sends a chat request and streams tokens via the channel.
	ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (<-chan StreamChunk, error)

	// GetProviderName returns the provider identifier (e.g., "openai", "anthropic").
	GetProviderName() string
}

// StreamChunk represents a single streamed token chunk.
type StreamChunk struct {
	Delta         string     `json:"delta"`
	ToolCallDelta *ToolCall  `json:"toolCallDelta,omitempty"`
	FinishReason  string     `json:"finishReason,omitempty"`
	Error         error      `json:"-"`
}
