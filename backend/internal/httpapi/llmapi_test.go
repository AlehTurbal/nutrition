package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// mockLLM is a deterministic Completer for tests (no network).
type mockLLM struct{}

func (mockLLM) Complete(_ context.Context, system, _ string) (string, error) {
	if strings.Contains(system, "cooking assistant") {
		return `{"name":"Тест","servings":1,"instructions":"x","meal_types":["lunch"],
			"ingredients":[{"name":"Курица","grams":200},{"name":"Соль","grams":3}]}`, nil
	}
	return `[{"needed":"Курица","matched":"Chicken 1kg","found":true}]`, nil
}

func TestStoreMatchFlow(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "store@example.com")

	chicken := createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)
	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/recipes", token, map[string]any{
		"name": "Блюдо", "servings": 1, "meal_types": []string{"lunch"},
		"ingredients": []map[string]any{{"product_id": chicken, "grams": 200}},
	})
	rid := int64(out["recipe"].(map[string]any)["id"].(float64))

	_, out = doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "P", "start_date": "2026-06-01", "end_date": "2026-06-02",
	})
	pid := int64(out["id"].(float64))
	doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans/"+itoa(pid)+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "recipe_id": rid, "servings": 1,
	})

	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/stores/match", token, map[string]any{
		"plan_id": pid, "store_text": "Chicken 1kg - 5.99",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("match status %d, body %v", resp.StatusCode, out)
	}
	items := out["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["found"] != true {
		t.Fatalf("expected one found match, got %v", items)
	}

	resp, out = doJSON(t, http.MethodGet, srv.URL+"/api/stores/matches/"+itoa(pid), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("latest status %d, body %v", resp.StatusCode, out)
	}
	if len(out["items"].([]any)) != 1 {
		t.Errorf("expected saved item, got %v", out["items"])
	}
}

func TestGenerateRecipeFlow(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "gen@example.com")
	createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)

	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/recipes/generate", token, map[string]any{
		"description": "что-то с курицей", "meal_types": []string{"lunch"}, "servings": 1,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("generate status %d, body %v", resp.StatusCode, out)
	}
	ings := out["ingredients"].([]any)
	if len(ings) != 2 {
		t.Fatalf("expected 2 ingredients, got %v", ings)
	}
	if ings[0].(map[string]any)["product_id"] == nil {
		t.Error("expected Курица to match a product")
	}
	if ings[1].(map[string]any)["product_id"] != nil {
		t.Error("expected Соль to be unmatched")
	}
}

func TestStoreMatchNoAPIKey(t *testing.T) {
	srv := newServerNoLLM(t)
	token := registerUser(t, srv.URL, "nokey@example.com")

	// A plan with at least one needed ingredient so the matcher actually
	// invokes the (unconfigured) LLM and surfaces ErrNoAPIKey.
	chicken := createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)
	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/recipes", token, map[string]any{
		"name": "Блюдо", "servings": 1, "meal_types": []string{"lunch"},
		"ingredients": []map[string]any{{"product_id": chicken, "grams": 200}},
	})
	rid := int64(out["recipe"].(map[string]any)["id"].(float64))

	_, out = doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "P", "start_date": "2026-06-01", "end_date": "2026-06-02",
	})
	pid := int64(out["id"].(float64))
	doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans/"+itoa(pid)+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "recipe_id": rid, "servings": 1,
	})

	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/api/stores/match", token, map[string]any{
		"plan_id": pid, "store_text": "anything",
	})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without API key, got %d", resp.StatusCode)
	}
}
