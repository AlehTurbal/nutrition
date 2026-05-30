package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/db"
	"github.com/alehturbal/nutrition/backend/internal/httpapi"
	"github.com/alehturbal/nutrition/backend/internal/products"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/users"
)

// These tests require a throwaway PostgreSQL database. Set NUTRITION_TEST_DATABASE_URL
// (the docker-compose stack provides one) to run them; otherwise they are skipped.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	dsn := os.Getenv("NUTRITION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set NUTRITION_TEST_DATABASE_URL to run integration tests")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Clean slate so the run is deterministic.
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS recipe_ingredients, recipes, products, weight_entries, profiles, users, schema_migrations CASCADE`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := users.NewStore(pool)
	tokens := auth.NewManager("test-secret", time.Hour)
	h := &httpapi.Handlers{
		Auth:     auth.NewService(store, tokens),
		Users:    store,
		Tokens:   tokens,
		Products: products.NewStore(pool),
		Recipes:  recipes.NewStore(pool),
	}
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	return srv
}

func doJSON(t *testing.T, method, url, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

func TestFullFlow(t *testing.T) {
	srv := newServer(t)

	// Register.
	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/auth/register", "",
		map[string]string{"email": "a@example.com", "password": "supersecret"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, body %v", resp.StatusCode, out)
	}
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatal("expected token from register")
	}

	// Set profile.
	resp, out = doJSON(t, http.MethodPut, srv.URL+"/api/profile", token, map[string]any{
		"sex": "male", "height_cm": 180, "age": 30,
		"activity_level": "moderate", "goal": "maintain",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put profile status = %d, body %v", resp.StatusCode, out)
	}

	// Add weight 80kg.
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/api/weights", token, map[string]any{"weight_kg": 80})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add weight status = %d", resp.StatusCode)
	}

	// Targets for 80kg.
	resp, out = doJSON(t, http.MethodGet, srv.URL+"/api/targets", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("targets status = %d, body %v", resp.StatusCode, out)
	}
	targets80 := out["targets"].(map[string]any)["calories"].(float64)

	// Change weight to 85kg -> targets must increase.
	doJSON(t, http.MethodPost, srv.URL+"/api/weights", token, map[string]any{"weight_kg": 85})
	_, out = doJSON(t, http.MethodGet, srv.URL+"/api/targets", token, nil)
	targets85 := out["targets"].(map[string]any)["calories"].(float64)

	if targets85 <= targets80 {
		t.Errorf("expected calories to rise with weight: %.1f -> %.1f", targets80, targets85)
	}
}

func TestAuthRequired(t *testing.T) {
	srv := newServer(t)
	resp, _ := doJSON(t, http.MethodGet, srv.URL+"/api/profile", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestDuplicateEmail(t *testing.T) {
	srv := newServer(t)
	body := map[string]string{"email": "dup@example.com", "password": "supersecret"}
	doJSON(t, http.MethodPost, srv.URL+"/api/auth/register", "", body)
	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/api/auth/register", "", body)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 on duplicate email, got %d", resp.StatusCode)
	}
}
