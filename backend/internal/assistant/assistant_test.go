package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alehturbal/nutrition/backend/internal/llm"
)

// scriptedLLM returns pre-canned responses in order and records the message
// histories it was called with.
type scriptedLLM struct {
	responses []llm.ToolResponse
	calls     int
	lastMsgs  []llm.Message
}

func (s *scriptedLLM) CreateMessage(_ context.Context, _ string, _ []llm.Tool, msgs []llm.Message) (llm.ToolResponse, error) {
	s.lastMsgs = msgs
	if s.calls >= len(s.responses) {
		// Default to a never-ending tool_use to exercise the cap if over-called.
		return llm.ToolResponse{StopReason: "tool_use", Content: []llm.ContentBlock{
			{Type: "tool_use", ID: "loop", Name: toolListProducts, Input: json.RawMessage(`{}`)},
		}}, nil
	}
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}

// stubData records which read tools ran and returns canned values.
type stubData struct {
	productsCalled bool
}

func (d *stubData) ListProducts(context.Context, int64) (any, error) {
	d.productsCalled = true
	return []map[string]any{{"id": 7, "name": "Овсянка"}}, nil
}
func (d *stubData) ListRecipes(context.Context, int64) (any, error) { return []any{}, nil }
func (d *stubData) GetTargets(context.Context, int64) (any, error) {
	return nil, errors.New("set profile first")
}
func (d *stubData) ListMealPlans(context.Context, int64) (any, error) { return []any{}, nil }

func toolUse(id, name, input string) llm.ContentBlock {
	return llm.ContentBlock{Type: "tool_use", ID: id, Name: name, Input: json.RawMessage(input)}
}

func TestReplyReadToolThenAnswer(t *testing.T) {
	llmMock := &scriptedLLM{responses: []llm.ToolResponse{
		{StopReason: "tool_use", Content: []llm.ContentBlock{toolUse("t1", toolListProducts, `{}`)}},
		{StopReason: "end_turn", Content: []llm.ContentBlock{{Type: "text", Text: "У тебя есть овсянка."}}},
	}}
	data := &stubData{}
	a := New(llmMock, data)

	text, proposals, err := a.Reply(context.Background(), 1, nil, "что у меня есть?")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !data.productsCalled {
		t.Error("expected list_products to be executed")
	}
	if text != "У тебя есть овсянка." {
		t.Errorf("text = %q", text)
	}
	if len(proposals) != 0 {
		t.Errorf("expected no proposals, got %v", proposals)
	}
	// The second call must include the tool_result fed back to the model.
	if !hasToolResult(llmMock.lastMsgs) {
		t.Error("expected a tool_result in the message history")
	}
}

func TestReplyMutatingToolBecomesProposal(t *testing.T) {
	input := `{"name":"Творог 5%","kcal100":121,"protein100":18,"fat100":5,"carbs100":3.3}`
	llmMock := &scriptedLLM{responses: []llm.ToolResponse{
		{StopReason: "tool_use", Content: []llm.ContentBlock{toolUse("t1", toolProposeProduct, input)}},
		{StopReason: "end_turn", Content: []llm.ContentBlock{{Type: "text", Text: "Предложил продукт, подтверди."}}},
	}}
	a := New(llmMock, &stubData{})

	text, proposals, err := a.Reply(context.Background(), 1, nil, "добавь творог")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(proposals))
	}
	if proposals[0].Type != "product" {
		t.Errorf("type = %q", proposals[0].Type)
	}
	if proposals[0].Payload["name"] != "Творог 5%" {
		t.Errorf("payload name = %v", proposals[0].Payload["name"])
	}
	if text == "" {
		t.Error("expected final text")
	}
}

func TestReplyIterationCap(t *testing.T) {
	// No scripted responses -> scriptedLLM keeps returning tool_use forever.
	a := New(&scriptedLLM{}, &stubData{})
	_, _, err := a.Reply(context.Background(), 1, nil, "loop")
	if !errors.Is(err, ErrMaxIterations) {
		t.Errorf("got %v, want ErrMaxIterations", err)
	}
}

func TestReplyPropagatesNoAPIKey(t *testing.T) {
	a := New(errLLM{llm.ErrNoAPIKey}, &stubData{})
	_, _, err := a.Reply(context.Background(), 1, nil, "hi")
	if !errors.Is(err, llm.ErrNoAPIKey) {
		t.Errorf("got %v, want ErrNoAPIKey", err)
	}
}

type errLLM struct{ err error }

func (e errLLM) CreateMessage(context.Context, string, []llm.Tool, []llm.Message) (llm.ToolResponse, error) {
	return llm.ToolResponse{}, e.err
}

func hasToolResult(msgs []llm.Message) bool {
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == "tool_result" {
				return true
			}
		}
	}
	return false
}

func TestParseProposal(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		input   string
		wantTyp string
		wantErr bool
	}{
		{"product", toolProposeProduct, `{"name":"Молоко","kcal100":60}`, "product", false},
		{"recipe", toolProposeRecipe, `{"name":"Каша","ingredients":[{"product_id":7,"grams":50}]}`, "recipe", false},
		{"missing name", toolProposeProduct, `{"kcal100":60}`, "", true},
		{"bad json", toolProposeProduct, `not json`, "", true},
		{"not a proposal tool", toolListProducts, `{"name":"x"}`, "", true},
		{"copy day", toolProposeCopyDay, `{"plan_id":1,"source_date":"2026-06-01","target_dates":["2026-06-02"]}`, "copy_day", false},
		{"copy day no targets", toolProposeCopyDay, `{"plan_id":1,"source_date":"2026-06-01","target_dates":[]}`, "", true},
		{"copy day no source", toolProposeCopyDay, `{"plan_id":1,"target_dates":["2026-06-02"]}`, "", true},
		{"add to plan", toolProposeAddToPlan, `{"plan_id":1,"day_date":"2026-06-01","meal_slot":"lunch","product_id":7,"grams":150}`, "add_to_plan", false},
		{"add to plan no grams", toolProposeAddToPlan, `{"plan_id":1,"day_date":"2026-06-01","meal_slot":"lunch","product_id":7}`, "", true},
		{"add to plan no product", toolProposeAddToPlan, `{"plan_id":1,"day_date":"2026-06-01","meal_slot":"lunch","grams":150}`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := parseProposal(tt.tool, json.RawMessage(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Type != tt.wantTyp {
				t.Errorf("type = %q, want %q", p.Type, tt.wantTyp)
			}
		})
	}
}

// A product proposal carrying product_id means "update this existing product",
// not "create a new one"; the id must survive into the payload so the UI can
// PUT it instead of POSTing a duplicate.
func TestParseProposalProductUpdateKeepsID(t *testing.T) {
	p, err := parseProposal(toolProposeProduct, json.RawMessage(`{"product_id":42,"name":"Молоко","kcal100":60}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != "product" {
		t.Fatalf("type = %q, want product", p.Type)
	}
	if got, ok := p.Payload["product_id"]; !ok || got != float64(42) {
		t.Errorf("payload product_id = %v (ok=%v), want 42", got, ok)
	}
}
