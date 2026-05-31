package recipes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alehturbal/nutrition/backend/internal/llm"
)

// Product is the minimal product shape the matcher needs (avoids importing the
// products package and creating a cycle).
type Product struct {
	ID   int64
	Name string
}

// GeneratedIngredient is an LLM-proposed ingredient as a name + grams.
type GeneratedIngredient struct {
	Name  string  `json:"name"`
	Grams float64 `json:"grams"`
}

// GeneratedRecipe is the LLM's structured recipe draft.
type GeneratedRecipe struct {
	Name         string                `json:"name"`
	Servings     int                   `json:"servings"`
	Instructions string                `json:"instructions"`
	MealTypes    []string              `json:"meal_types"`
	Ingredients  []GeneratedIngredient `json:"ingredients"`
}

// MatchedIngredient pairs a generated ingredient with a user product id if one
// matches by name (nil ProductID = needs to be created first).
type MatchedIngredient struct {
	Name      string  `json:"name"`
	Grams     float64 `json:"grams"`
	ProductID *int64  `json:"product_id"`
}

const generateSystemPrompt = `You are a cooking assistant. Given a dish description, produce a single recipe.
Respond ONLY with a JSON object shaped:
{"name": "...", "servings": <int>, "instructions": "...", "meal_types": ["breakfast"|"lunch"|"dinner"|"snack", ...], "ingredients": [{"name": "...", "grams": <number>}, ...]}
Use the same language as the description. meal_types may be empty. Give realistic gram weights.`

// Generator turns a free-text description into a recipe draft via a Completer.
type Generator struct {
	llm llm.Completer
}

// NewGenerator wraps a Completer.
func NewGenerator(c llm.Completer) *Generator {
	return &Generator{llm: c}
}

// Generate asks the LLM for a recipe draft. slots and servings are hints.
func (g *Generator) Generate(ctx context.Context, description string, slots []string, servings int) (GeneratedRecipe, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Dish description: %s\n", description)
	if len(slots) > 0 {
		fmt.Fprintf(&b, "Preferred meal types: %s\n", strings.Join(slots, ", "))
	}
	if servings > 0 {
		fmt.Fprintf(&b, "Servings: %d\n", servings)
	}

	raw, err := g.llm.Complete(ctx, generateSystemPrompt, b.String())
	if err != nil {
		return GeneratedRecipe{}, err
	}
	return parseGenerated(raw)
}

func parseGenerated(raw string) (GeneratedRecipe, error) {
	jsonStr, err := llm.ExtractJSON(raw)
	if err != nil {
		return GeneratedRecipe{}, fmt.Errorf("parse recipe: %w", err)
	}
	var out GeneratedRecipe
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return GeneratedRecipe{}, fmt.Errorf("unmarshal recipe: %w", err)
	}
	return out, nil
}

// MatchIngredientsToProducts links each generated ingredient to a user product
// by case-insensitive name match, leaving ProductID nil when none matches.
func MatchIngredientsToProducts(gen GeneratedRecipe, owned []Product) []MatchedIngredient {
	byName := make(map[string]int64, len(owned))
	for _, p := range owned {
		byName[strings.ToLower(strings.TrimSpace(p.Name))] = p.ID
	}
	out := make([]MatchedIngredient, len(gen.Ingredients))
	for i, ing := range gen.Ingredients {
		m := MatchedIngredient{Name: ing.Name, Grams: ing.Grams}
		if id, ok := byName[strings.ToLower(strings.TrimSpace(ing.Name))]; ok {
			idCopy := id
			m.ProductID = &idCopy
		}
		out[i] = m
	}
	return out
}
