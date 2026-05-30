package httpapi_test

import (
	"math"
	"net/http"
	"strconv"
	"testing"
)

// registerUser returns a fresh auth token.
func registerUser(t *testing.T, baseURL, email string) string {
	t.Helper()
	resp, out := doJSON(t, http.MethodPost, baseURL+"/api/auth/register", "",
		map[string]string{"email": email, "password": "supersecret"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s: status %d", email, resp.StatusCode)
	}
	return out["token"].(string)
}

func TestProductCRUD(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "products@example.com")

	// Create with БЖУ.
	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/products", token, map[string]any{
		"name": "Куриная грудка", "category": "Мясо",
		"kcal100": 165, "protein100": 31, "fat100": 3.6, "carbs100": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create product: status %d, body %v", resp.StatusCode, out)
	}
	id := int64(out["id"].(float64))

	// Get.
	resp, out = doJSON(t, http.MethodGet, srv.URL+"/api/products/"+itoa(id), token, nil)
	if resp.StatusCode != http.StatusOK || out["name"] != "Куриная грудка" {
		t.Fatalf("get product: status %d, body %v", resp.StatusCode, out)
	}

	// List.
	resp, _ = doJSON(t, http.MethodGet, srv.URL+"/api/products", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list products: status %d", resp.StatusCode)
	}

	// Update БЖУ.
	resp, out = doJSON(t, http.MethodPut, srv.URL+"/api/products/"+itoa(id), token, map[string]any{
		"name": "Куриная грудка", "kcal100": 170, "protein100": 32, "fat100": 4, "carbs100": 0,
	})
	if resp.StatusCode != http.StatusOK || out["kcal100"].(float64) != 170 {
		t.Fatalf("update product: status %d, body %v", resp.StatusCode, out)
	}

	// Delete then 404.
	resp, _ = doJSON(t, http.MethodDelete, srv.URL+"/api/products/"+itoa(id), token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete product: status %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodGet, srv.URL+"/api/products/"+itoa(id), token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
}

func TestRecipeMacrosAndMealTypes(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "recipes@example.com")

	chicken := createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)
	rice := createProduct(t, srv.URL, token, "Рис", 130, 2.7, 0.3, 28)

	// 200g chicken + 150g rice, 2 servings, lunch/dinner.
	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/recipes", token, map[string]any{
		"name": "Курица с рисом", "servings": 2,
		"meal_types": []string{"lunch", "dinner"},
		"ingredients": []map[string]any{
			{"product_id": chicken, "grams": 200},
			{"product_id": rice, "grams": 150},
		},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create recipe: status %d, body %v", resp.StatusCode, out)
	}

	macros := out["macros"].(map[string]any)
	if !macros["complete"].(bool) {
		t.Error("expected complete macros")
	}
	approxEq(t, "total kcal", macros["kcal"].(float64), 525)       // 165*2 + 130*1.5
	approxEq(t, "total protein", macros["protein"].(float64), 66.05)

	perServing := out["macros_per_serving"].(map[string]any)
	approxEq(t, "per-serving kcal", perServing["kcal"].(float64), 262.5)

	rec := out["recipe"].(map[string]any)
	mt := rec["meal_types"].([]any)
	if len(mt) != 2 {
		t.Errorf("expected 2 meal types, got %v", mt)
	}
}

func TestRecipeRejectsForeignProduct(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "owner@example.com")

	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/api/recipes", token, map[string]any{
		"name": "Bad", "ingredients": []map[string]any{{"product_id": 999999, "grams": 100}},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown product, got %d", resp.StatusCode)
	}
}

// helpers

func createProduct(t *testing.T, baseURL, token, name string, kcal, p, f, c float64) int64 {
	t.Helper()
	resp, out := doJSON(t, http.MethodPost, baseURL+"/api/products", token, map[string]any{
		"name": name, "kcal100": kcal, "protein100": p, "fat100": f, "carbs100": c,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create product %s: status %d", name, resp.StatusCode)
	}
	return int64(out["id"].(float64))
}

func approxEq(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
