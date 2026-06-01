package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Tool is an Anthropic tool definition advertised to the model.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ContentBlock is a single block within a message. Depending on Type ("text",
// "tool_use", "tool_result") a different subset of fields is populated; the
// omitempty tags keep the wire shape valid for each variant.
type ContentBlock struct {
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// tool_use (assistant → us)
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result (us → assistant)
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

// Message is one conversational turn with structured content blocks.
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ToolResponse is the model's reply to a CreateMessage call.
type ToolResponse struct {
	StopReason string
	Content    []ContentBlock
}

// ToolCaller runs one tool-use turn over a full message history. *Client
// implements it against the Anthropic API; tests use a fake.
type ToolCaller interface {
	CreateMessage(ctx context.Context, system string, tools []Tool, msgs []Message) (ToolResponse, error)
}

type toolRequestBody struct {
	Model     string     `json:"model"`
	MaxTokens int        `json:"max_tokens"`
	System    []sysBlock `json:"system"`
	Tools     []Tool     `json:"tools,omitempty"`
	Messages  []Message  `json:"messages"`
}

type toolResponseBody struct {
	StopReason string         `json:"stop_reason"`
	Content    []ContentBlock `json:"content"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// CreateMessage runs one turn of a tool-use conversation: it sends the system
// prompt, tool definitions and full message history, and returns the model's
// stop reason and content blocks (which may include tool_use). A nil *Client is
// the "not configured" state and returns ErrNoAPIKey.
func (c *Client) CreateMessage(ctx context.Context, system string, tools []Tool, msgs []Message) (ToolResponse, error) {
	if c == nil {
		return ToolResponse{}, ErrNoAPIKey
	}

	body, err := json.Marshal(toolRequestBody{
		Model:     c.model,
		MaxTokens: maxTokens,
		System: []sysBlock{{
			Type:         "text",
			Text:         system,
			CacheControl: map[string]any{"type": "ephemeral"},
		}},
		Tools:    tools,
		Messages: msgs,
	})
	if err != nil {
		return ToolResponse{}, err
	}

	raw, status, err := c.post(ctx, body)
	if err != nil {
		return ToolResponse{}, err
	}

	var parsed toolResponseBody
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ToolResponse{}, fmt.Errorf("decode anthropic response: %w", err)
	}
	if status != http.StatusOK {
		msg := "non-200 from anthropic"
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return ToolResponse{}, fmt.Errorf("anthropic %d: %s", status, msg)
	}

	return ToolResponse{StopReason: parsed.StopReason, Content: parsed.Content}, nil
}
