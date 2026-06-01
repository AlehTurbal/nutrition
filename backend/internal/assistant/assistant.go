// Package assistant runs a Claude tool-use loop over the Phase 4 llm.Client.
// Read tools are executed immediately against the user's data; mutating tools
// are NOT executed — they are converted into proposals the user confirms in the
// UI, which then calls the existing CRUD endpoints.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alehturbal/nutrition/backend/internal/llm"
)

// maxIterations caps the tool-use loop so a misbehaving model cannot spin
// forever.
const maxIterations = 8

// ErrMaxIterations is returned when the loop does not settle on a final answer.
var ErrMaxIterations = errors.New("assistant did not finish within the step limit")

// Turn is one persisted message in the visible dialog.
type Turn struct {
	Role string // "user" | "assistant"
	Text string
}

// Proposal is a previewed mutation the user confirms. Payload is shaped exactly
// like the body of the matching CRUD endpoint (POST /api/products or
// /api/recipes).
type Proposal struct {
	Type    string         `json:"type"` // "product" | "recipe"
	Payload map[string]any `json:"payload"`
}

// DataSource backs the read tools, scoped to a user. Implementations return
// JSON-marshalable values (typically the same structs the HTTP API returns).
type DataSource interface {
	ListProducts(ctx context.Context, userID int64) (any, error)
	ListRecipes(ctx context.Context, userID int64) (any, error)
	GetTargets(ctx context.Context, userID int64) (any, error)
}

// Assistant pairs a tool-capable LLM with a per-user data source.
type Assistant struct {
	llm  llm.ToolCaller
	data DataSource
}

// New wraps a ToolCaller and DataSource.
func New(c llm.ToolCaller, data DataSource) *Assistant {
	return &Assistant{llm: c, data: data}
}

// Reply continues a conversation. history is the prior visible dialog; userMsg
// is the new user message. It runs the tool-use loop, executing read tools and
// collecting mutating-tool calls as proposals, and returns the assistant's final
// text plus any proposals. An llm.ErrNoAPIKey error propagates so handlers can
// return 503.
func (a *Assistant) Reply(ctx context.Context, userID int64, history []Turn, userMsg string) (string, []Proposal, error) {
	msgs := make([]llm.Message, 0, len(history)+1)
	for _, t := range history {
		msgs = append(msgs, llm.Message{
			Role:    t.Role,
			Content: []llm.ContentBlock{{Type: "text", Text: t.Text}},
		})
	}
	msgs = append(msgs, llm.Message{
		Role:    "user",
		Content: []llm.ContentBlock{{Type: "text", Text: userMsg}},
	})

	var proposals []Proposal
	for i := 0; i < maxIterations; i++ {
		resp, err := a.llm.CreateMessage(ctx, systemPrompt, toolDefs, msgs)
		if err != nil {
			return "", nil, err
		}
		msgs = append(msgs, llm.Message{Role: "assistant", Content: resp.Content})

		if resp.StopReason != "tool_use" {
			return collectText(resp.Content), proposals, nil
		}

		results := make([]llm.ContentBlock, 0)
		for _, b := range resp.Content {
			if b.Type != "tool_use" {
				continue
			}
			switch b.Name {
			case toolListProducts, toolListRecipes, toolGetTargets:
				results = append(results, llm.ContentBlock{
					Type:      "tool_result",
					ToolUseID: b.ID,
					Content:   a.runRead(ctx, userID, b.Name),
				})
			case toolProposeProduct, toolProposeRecipe:
				p, err := parseProposal(b.Name, b.Input)
				if err != nil {
					results = append(results, toolErr(b.ID, err))
					continue
				}
				proposals = append(proposals, p)
				results = append(results, llm.ContentBlock{
					Type:      "tool_result",
					ToolUseID: b.ID,
					Content:   "Proposal shown to the user for confirmation. Do not assume it was applied.",
				})
			default:
				results = append(results, toolErr(b.ID, fmt.Errorf("unknown tool %q", b.Name)))
			}
		}
		msgs = append(msgs, llm.Message{Role: "user", Content: results})
	}
	return "", proposals, ErrMaxIterations
}

// runRead executes a read tool and returns its result as a JSON string for the
// model. Errors are surfaced to the model rather than aborting the loop.
func (a *Assistant) runRead(ctx context.Context, userID int64, name string) string {
	var (
		v   any
		err error
	)
	switch name {
	case toolListProducts:
		v, err = a.data.ListProducts(ctx, userID)
	case toolListRecipes:
		v, err = a.data.ListRecipes(ctx, userID)
	case toolGetTargets:
		v, err = a.data.GetTargets(ctx, userID)
	}
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"could not encode result"}`
	}
	return string(b)
}

func collectText(blocks []llm.ContentBlock) string {
	var out string
	for _, b := range blocks {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return out
}

func toolErr(toolUseID string, err error) llm.ContentBlock {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return llm.ContentBlock{Type: "tool_result", ToolUseID: toolUseID, Content: string(b)}
}
