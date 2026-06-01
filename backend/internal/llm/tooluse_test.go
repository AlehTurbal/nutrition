package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCreateMessageToolUse(t *testing.T) {
	var gotBody map[string]any
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		resp := `{"stop_reason":"tool_use","content":[
			{"type":"text","text":"let me check"},
			{"type":"tool_use","id":"tu_1","name":"list_products","input":{}}
		]}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(resp)),
			Header:     make(http.Header),
		}, nil
	})

	c := New("secret", "claude-opus-4-8")
	c.http = &http.Client{Transport: rt}

	tools := []Tool{{Name: "list_products", Description: "list", InputSchema: map[string]any{"type": "object"}}}
	msgs := []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}}}

	resp, err := c.CreateMessage(context.Background(), "SYS", tools, msgs)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q", resp.StopReason)
	}
	if len(resp.Content) != 2 || resp.Content[1].Type != "tool_use" || resp.Content[1].Name != "list_products" {
		t.Fatalf("unexpected content: %+v", resp.Content)
	}
	if resp.Content[1].ID != "tu_1" {
		t.Errorf("tool_use id = %q", resp.Content[1].ID)
	}

	// The request must carry tools and the message history.
	if gotBody["tools"] == nil {
		t.Error("expected tools in request body")
	}
	if msgsOut, ok := gotBody["messages"].([]any); !ok || len(msgsOut) != 1 {
		t.Errorf("messages not sent: %v", gotBody["messages"])
	}
}

func TestCreateMessageNilClient(t *testing.T) {
	var c *Client
	if _, err := c.CreateMessage(context.Background(), "s", nil, nil); err != ErrNoAPIKey {
		t.Errorf("got %v, want ErrNoAPIKey", err)
	}
}
