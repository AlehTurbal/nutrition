package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets a test stand in for http transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientComplete(t *testing.T) {
	var gotBody map[string]any
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-api-key") != "secret" {
			t.Errorf("missing api key header")
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		resp := `{"content":[{"type":"text","text":"hello world"}]}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(resp)),
			Header:     make(http.Header),
		}, nil
	})

	c := New("secret", "claude-opus-4-8")
	c.http = &http.Client{Transport: rt}

	out, err := c.Complete(context.Background(), "SYS", "USER")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != "hello world" {
		t.Errorf("got %q, want %q", out, "hello world")
	}
	if gotBody["model"] != "claude-opus-4-8" {
		t.Errorf("model = %v", gotBody["model"])
	}
	sys, ok := gotBody["system"].([]any)
	if !ok || len(sys) == 0 {
		t.Fatalf("system not an array: %v", gotBody["system"])
	}
	block := sys[0].(map[string]any)
	if block["text"] != "SYS" {
		t.Errorf("system text = %v", block["text"])
	}
	if block["cache_control"] == nil {
		t.Error("expected cache_control on system block")
	}
}

func TestClientHTTPError(t *testing.T) {
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 429,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := New("secret", "claude-opus-4-8")
	c.http = &http.Client{Transport: rt}
	if _, err := c.Complete(context.Background(), "s", "u"); err == nil {
		t.Error("expected error on non-200")
	}
}

func TestNilClientErrNoAPIKey(t *testing.T) {
	var c *Client // nil => not configured
	if _, err := c.Complete(context.Background(), "s", "u"); err != ErrNoAPIKey {
		t.Errorf("got %v, want ErrNoAPIKey", err)
	}
}
