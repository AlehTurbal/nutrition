package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/alehturbal/nutrition/backend/internal/llm"
)

// scriptedToolLLM returns canned tool-use responses in order, then end_turn.
type scriptedToolLLM struct {
	responses []llm.ToolResponse
	calls     int
}

func (s *scriptedToolLLM) CreateMessage(context.Context, string, []llm.Tool, []llm.Message) (llm.ToolResponse, error) {
	if s.calls >= len(s.responses) {
		return llm.ToolResponse{StopReason: "end_turn", Content: []llm.ContentBlock{{Type: "text", Text: "Готово."}}}, nil
	}
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}

func registerToken(t *testing.T, url, email string) string {
	t.Helper()
	resp, out := doJSON(t, http.MethodPost, url+"/api/auth/register", "",
		map[string]string{"email": email, "password": "supersecret"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, body %v", resp.StatusCode, out)
	}
	return out["token"].(string)
}

func TestChatProposeThenApply(t *testing.T) {
	caller := &scriptedToolLLM{responses: []llm.ToolResponse{
		{StopReason: "tool_use", Content: []llm.ContentBlock{{
			Type: "tool_use", ID: "t1", Name: "propose_product",
			Input: json.RawMessage(`{"name":"Творог 5%","kcal100":121,"protein100":18,"fat100":5,"carbs100":3.3}`),
		}}},
		{StopReason: "end_turn", Content: []llm.ContentBlock{{Type: "text", Text: "Предложил творог, подтверди."}}},
	}}
	srv, _ := newServerWithAssistant(t, caller)
	token := registerToken(t, srv.URL, "chat@example.com")

	// Create a thread.
	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/chat/threads", token, map[string]any{"title": "Test"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create thread = %d, body %v", resp.StatusCode, out)
	}
	threadID := int64(out["id"].(float64))

	// Send a message -> assistant proposes a product.
	resp, out = doJSON(t, http.MethodPost,
		srv.URL+"/api/chat/threads/"+itoa(threadID)+"/messages", token,
		map[string]any{"content": "добавь творог"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post message = %d, body %v", resp.StatusCode, out)
	}
	proposals, _ := out["proposals"].([]any)
	if len(proposals) != 1 {
		t.Fatalf("expected 1 proposal, got %v", out["proposals"])
	}
	p0 := proposals[0].(map[string]any)
	if p0["type"] != "product" {
		t.Errorf("proposal type = %v", p0["type"])
	}
	payload := p0["payload"].(map[string]any)
	if payload["name"] != "Творог 5%" {
		t.Errorf("payload name = %v", payload["name"])
	}

	// The product must NOT have been created by the proposal.
	if n := countProducts(t, srv.URL, token); n != 0 {
		t.Fatalf("expected 0 products before apply, got %d", n)
	}

	// Apply the proposal by posting its payload to the existing CRUD endpoint.
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/api/products", token, payload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("apply proposal (create product) = %d", resp.StatusCode)
	}
	if n := countProducts(t, srv.URL, token); n != 1 {
		t.Fatalf("expected 1 product after apply, got %d", n)
	}

	// The visible dialog (user + assistant) is persisted.
	if n := countMessages(t, srv.URL, token, threadID); n != 2 {
		t.Fatalf("expected 2 messages, got %d", n)
	}
}

func TestChatNoAPIKeyReturns503(t *testing.T) {
	srv := newServerNoLLM(t)
	token := registerToken(t, srv.URL, "nokey@example.com")

	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/chat/threads", token, map[string]any{})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create thread = %d", resp.StatusCode)
	}
	threadID := int64(out["id"].(float64))

	resp, _ = doJSON(t, http.MethodPost,
		srv.URL+"/api/chat/threads/"+itoa(threadID)+"/messages", token,
		map[string]any{"content": "привет"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without API key, got %d", resp.StatusCode)
	}
}

func TestChatRenameThread(t *testing.T) {
	srv, _ := newServerWithAssistant(t, &scriptedToolLLM{})
	token := registerToken(t, srv.URL, "rename@example.com")

	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/chat/threads", token, map[string]any{})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create thread = %d", resp.StatusCode)
	}
	threadID := int64(out["id"].(float64))

	// Rename succeeds and returns the new title.
	resp, out = doJSON(t, http.MethodPut,
		srv.URL+"/api/chat/threads/"+itoa(threadID), token,
		map[string]any{"title": "Завтраки"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename = %d, body %v", resp.StatusCode, out)
	}
	if out["title"] != "Завтраки" {
		t.Errorf("title = %v, want Завтраки", out["title"])
	}

	// Empty title is rejected.
	resp, _ = doJSON(t, http.MethodPut,
		srv.URL+"/api/chat/threads/"+itoa(threadID), token,
		map[string]any{"title": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty rename = %d, want 400", resp.StatusCode)
	}

	// Another user cannot rename this thread.
	other := registerToken(t, srv.URL, "rename-other@example.com")
	resp, _ = doJSON(t, http.MethodPut,
		srv.URL+"/api/chat/threads/"+itoa(threadID), other,
		map[string]any{"title": "Hijack"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-user rename = %d, want 404", resp.StatusCode)
	}
}

func TestChatAutoTitleFromFirstMessage(t *testing.T) {
	srv, _ := newServerWithAssistant(t, &scriptedToolLLM{})
	token := registerToken(t, srv.URL, "autotitle@example.com")

	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/chat/threads", token, map[string]any{})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create thread = %d", resp.StatusCode)
	}
	threadID := int64(out["id"].(float64))
	if out["title"] != "" {
		t.Fatalf("new thread title = %v, want empty", out["title"])
	}

	// First message names the thread.
	resp, _ = doJSON(t, http.MethodPost,
		srv.URL+"/api/chat/threads/"+itoa(threadID)+"/messages", token,
		map[string]any{"content": "добавь творог 5%"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first message = %d", resp.StatusCode)
	}
	if got := threadTitle(t, srv.URL, token, threadID); got != "добавь творог 5%" {
		t.Fatalf("auto title = %q, want %q", got, "добавь творог 5%")
	}

	// A second message must not overwrite the title.
	resp, _ = doJSON(t, http.MethodPost,
		srv.URL+"/api/chat/threads/"+itoa(threadID)+"/messages", token,
		map[string]any{"content": "и ещё кефир"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second message = %d", resp.StatusCode)
	}
	if got := threadTitle(t, srv.URL, token, threadID); got != "добавь творог 5%" {
		t.Errorf("title after second message = %q, want unchanged", got)
	}
}

// threadTitle fetches the user's threads and returns the title of one by id.
func threadTitle(t *testing.T, baseURL, token string, id int64) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/chat/threads", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	var threads []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&threads); err != nil {
		t.Fatalf("decode threads: %v", err)
	}
	for _, th := range threads {
		if th.ID == id {
			return th.Title
		}
	}
	t.Fatalf("thread %d not found", id)
	return ""
}

// countProducts / countMessages decode JSON-array responses (doJSON only decodes
// objects).
func countProducts(t *testing.T, baseURL, token string) int {
	t.Helper()
	return countArray(t, http.MethodGet, baseURL+"/api/products", token)
}

func countMessages(t *testing.T, baseURL, token string, threadID int64) int {
	t.Helper()
	return countArray(t, http.MethodGet, baseURL+"/api/chat/threads/"+itoa(threadID)+"/messages", token)
}

func countArray(t *testing.T, method, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	var arr []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("decode array: %v", err)
	}
	return len(arr)
}
