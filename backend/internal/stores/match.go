// Package stores matches a meal plan's needed ingredients against a store's
// pasted assortment text using an LLM, and persists the proposals.
package stores

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alehturbal/nutrition/backend/internal/llm"
)

// Needed is one ingredient the plan requires, with the total grams to buy.
type Needed struct {
	Name  string  `json:"name"`
	Grams float64 `json:"grams"`
}

// Match is the LLM's proposal for one needed ingredient.
type Match struct {
	Needed  string `json:"needed"`
	Matched string `json:"matched"`
	Found   bool   `json:"found"`
}

const systemPrompt = `You match a shopping list of needed ingredients against the raw text of a grocery store's assortment (which may be in any language).
For each needed ingredient, find the single best matching product line from the store text.
Respond ONLY with a JSON array; one object per needed ingredient, in the same order, shaped:
[{"needed": "<the needed name, verbatim>", "matched": "<store product text, or empty string>", "found": <true|false>}]
Set found=false and matched="" when no reasonable match exists. Do not invent products that are not in the store text.`

// Service runs ingredient↔store matching through a Completer.
type Service struct {
	llm llm.Completer
}

// NewService wraps a Completer.
func NewService(c llm.Completer) *Service {
	return &Service{llm: c}
}

// Match asks the LLM to pair each needed ingredient with a store product.
func (s *Service) Match(ctx context.Context, needed []Needed, storeText string) ([]Match, error) {
	if len(needed) == 0 {
		return []Match{}, nil
	}

	var b strings.Builder
	b.WriteString("Needed ingredients:\n")
	for _, n := range needed {
		fmt.Fprintf(&b, "- %s (%.0f g)\n", n.Name, n.Grams)
	}
	b.WriteString("\nStore assortment text:\n")
	b.WriteString(storeText)

	raw, err := s.llm.Complete(ctx, systemPrompt, b.String())
	if err != nil {
		return nil, err
	}
	return parseMatches(raw)
}

// parseMatches extracts the JSON array of matches from the model output.
func parseMatches(raw string) ([]Match, error) {
	jsonStr, err := llm.ExtractJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse matches: %w", err)
	}
	var out []Match
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return nil, fmt.Errorf("unmarshal matches: %w", err)
	}
	return out, nil
}
