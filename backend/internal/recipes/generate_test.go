package recipes

import (
	"context"
	"strings"
	"testing"
)

type fakeCompleter struct {
	reply string
	err   error
	user  string
}

func (f *fakeCompleter) Complete(_ context.Context, _, user string) (string, error) {
	f.user = user
	return f.reply, f.err
}

func TestGenerateParses(t *testing.T) {
	fc := &fakeCompleter{reply: `{"name":"Овсянка","servings":1,"instructions":"Сварить",
"meal_types":["breakfast"],
"ingredients":[{"name":"Овсяные хлопья","grams":60},{"name":"Молоко","grams":200}]}`}
	g := NewGenerator(fc)

	got, err := g.Generate(context.Background(), "овсянка на завтрак", []string{"breakfast"}, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(fc.user, "овсянка на завтрак") {
		t.Errorf("prompt missing description: %q", fc.user)
	}
	if got.Name != "Овсянка" || got.Servings != 1 {
		t.Errorf("recipe meta wrong: %+v", got)
	}
	if len(got.Ingredients) != 2 || got.Ingredients[0].Name != "Овсяные хлопья" || got.Ingredients[0].Grams != 60 {
		t.Errorf("ingredients wrong: %+v", got.Ingredients)
	}
	if len(got.MealTypes) != 1 || got.MealTypes[0] != "breakfast" {
		t.Errorf("meal types wrong: %+v", got.MealTypes)
	}
}

func TestMatchIngredientsToProducts(t *testing.T) {
	owned := []Product{
		{ID: 1, Name: "Молоко"},
		{ID: 2, Name: "Овсяные хлопья"},
	}
	gen := GeneratedRecipe{
		Ingredients: []GeneratedIngredient{
			{Name: "молоко", Grams: 200},
			{Name: "Овсяные хлопья", Grams: 60},
			{Name: "Соль", Grams: 2},
		},
	}
	matched := MatchIngredientsToProducts(gen, owned)
	if matched[0].ProductID == nil || *matched[0].ProductID != 1 {
		t.Errorf("milk should match product 1: %+v", matched[0])
	}
	if matched[1].ProductID == nil || *matched[1].ProductID != 2 {
		t.Errorf("oats should match product 2: %+v", matched[1])
	}
	if matched[2].ProductID != nil {
		t.Errorf("salt should be unmatched: %+v", matched[2])
	}
}
