// Package recipes stores dishes (recipes) with their ingredients and computes
// the БЖУ (protein/fat/carbs) and calories of a dish from its ingredients.
package recipes

// IngredientMacro carries one ingredient's weight and the per-100g nutrition of
// its product. The per-100g values are pointers because a product's БЖУ may not
// be filled in yet (nil).
type IngredientMacro struct {
	Grams      float64
	Kcal100    *float64
	Protein100 *float64
	Fat100     *float64
	Carbs100   *float64
}

// Macros is the aggregated nutrition of a whole recipe.
type Macros struct {
	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein"`
	Fat     float64 `json:"fat"`
	Carbs   float64 `json:"carbs"`
	// Complete is false when at least one ingredient is missing any per-100g
	// value, meaning the totals understate the real nutrition.
	Complete bool `json:"complete"`
}

// ComputeMacros sums the contribution of every ingredient. A missing per-100g
// value is treated as zero but flips Complete to false so callers can warn the
// user that some products still need their БЖУ filled in.
func ComputeMacros(ings []IngredientMacro) Macros {
	m := Macros{Complete: true}
	for _, ing := range ings {
		factor := ing.Grams / 100.0
		m.Kcal += value(ing.Kcal100, &m.Complete) * factor
		m.Protein += value(ing.Protein100, &m.Complete) * factor
		m.Fat += value(ing.Fat100, &m.Complete) * factor
		m.Carbs += value(ing.Carbs100, &m.Complete) * factor
	}
	return m
}

func value(p *float64, complete *bool) float64 {
	if p == nil {
		*complete = false
		return 0
	}
	return *p
}
