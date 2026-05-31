package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNoAPIKey signals that LLM features are unconfigured (no API key). Handlers
// translate this to HTTP 503.
var ErrNoAPIKey = errors.New("LLM not configured: missing ANTHROPIC_API_KEY")

const (
	apiURL        = "https://api.anthropic.com/v1/messages"
	apiVersion    = "2023-06-01"
	maxTokens     = 4096
	requestTimout = 60 * time.Second
)

// Completer turns a system + user prompt into a single text completion. The llm
// package's *Client implements it; tests use a fake.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Client calls the Anthropic Messages API. A nil *Client is the "not
// configured" state and returns ErrNoAPIKey from Complete.
type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

// New returns a configured client. If apiKey is empty, New returns nil so the
// caller can treat "no key" uniformly via the nil receiver.
func New(apiKey, model string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: requestTimout},
	}
}

type sysBlock struct {
	Type         string         `json:"type"`
	Text         string         `json:"text"`
	CacheControl map[string]any `json:"cache_control,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type requestBody struct {
	Model     string     `json:"model"`
	MaxTokens int        `json:"max_tokens"`
	System    []sysBlock `json:"system"`
	Messages  []message  `json:"messages"`
}

type responseBody struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends one system+user turn and returns the concatenated text. The
// system prompt is sent as a cacheable block (ephemeral prompt caching).
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	if c == nil {
		return "", ErrNoAPIKey
	}

	body, err := json.Marshal(requestBody{
		Model:     c.model,
		MaxTokens: maxTokens,
		System: []sysBlock{{
			Type:         "text",
			Text:         system,
			CacheControl: map[string]any{"type": "ephemeral"},
		}},
		Messages: []message{{Role: "user", Content: user}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read anthropic response: %w", err)
	}

	var parsed responseBody
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode anthropic response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := "non-200 from anthropic"
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return "", fmt.Errorf("anthropic %d: %s", resp.StatusCode, msg)
	}

	var out strings.Builder
	for _, b := range parsed.Content {
		if b.Type == "text" {
			out.WriteString(b.Text)
		}
	}
	return out.String(), nil
}
