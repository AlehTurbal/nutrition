package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alehturbal/nutrition/backend/internal/assistant"
	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/chat"
	"github.com/alehturbal/nutrition/backend/internal/db"
	"github.com/alehturbal/nutrition/backend/internal/httpapi"
	"github.com/alehturbal/nutrition/backend/internal/llm"
	"github.com/alehturbal/nutrition/backend/internal/mealplans"
	"github.com/alehturbal/nutrition/backend/internal/products"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/stores"
	"github.com/alehturbal/nutrition/backend/internal/users"
)

// These tests require a throwaway PostgreSQL database whose name contains
// "test" — see bootstrapDB. Set NUTRITION_TEST_DATABASE_URL (the docker-compose
// stack provides one) to run them; otherwise they are skipped.

// ensureTestDatabase refuses to touch a database whose name does not look like a
// test database (guarding the dev DB), then creates the target database if it
// does not yet exist by connecting to the "postgres" maintenance database.
func ensureTestDatabase(ctx context.Context, dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	name := strings.TrimPrefix(u.Path, "/")
	if !strings.Contains(name, "test") {
		return fmt.Errorf("refusing to run destructive tests against database %q: its name must contain \"test\"", name)
	}

	admin := *u
	admin.Path = "/postgres"
	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		return fmt.Errorf("connect maintenance db: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P04" { // duplicate_database
			return nil
		}
		return fmt.Errorf("create database %q: %w", name, err)
	}
	return nil
}

// bootstrapDB connects to the isolated test database (creating it if needed) and
// returns a pool against a freshly migrated, empty schema.
func bootstrapDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("NUTRITION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set NUTRITION_TEST_DATABASE_URL to run integration tests")
	}

	ctx := context.Background()
	if err := ensureTestDatabase(ctx, dsn); err != nil {
		t.Fatalf("ensure test database: %v", err)
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Clean slate so the run is deterministic.
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS chat_messages, chat_threads, store_match_items, store_matches, meal_plan_items, meal_plans, recipe_ingredients, recipes, products, weight_entries, profiles, users, schema_migrations CASCADE`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// handlersFor assembles the HTTP handlers over an existing pool, parameterized
// by the LLM-backed dependencies (store matcher, recipe generator, assistant
// tool caller) so tests can swap mocks or the unconfigured nil client.
func handlersFor(pool *pgxpool.Pool, matcher httpapi.StoreMatcher, gen httpapi.RecipeGenerator, caller llm.ToolCaller) *httpapi.Handlers {
	store := users.NewStore(pool)
	productStore := products.NewStore(pool)
	recipeStore := recipes.NewStore(pool)
	tokens := auth.NewManager("test-secret", time.Hour)
	return &httpapi.Handlers{
		Auth:         auth.NewService(store, tokens),
		Users:        store,
		Tokens:       tokens,
		Products:     productStore,
		Recipes:      recipeStore,
		MealPlans:    mealplans.NewStore(pool),
		StoreMatcher: matcher,
		StoreStore:   stores.NewStore(pool),
		RecipeGen:    gen,
		Chat:         chat.NewStore(pool),
		Assistant:    assistant.New(caller, httpapi.NewStoreData(store, productStore, recipeStore)),
	}
}

func newServer(t *testing.T) *httptest.Server {
	srv, _ := newServerWithPool(t)
	return srv
}

// newServerWithPool is like newServer but also returns the underlying pool so a
// test can manipulate the database directly.
func newServerWithPool(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := bootstrapDB(t)
	h := handlersFor(pool, stores.NewService(mockLLM{}), recipes.NewGenerator(mockLLM{}), endTurnLLM{})
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	return srv, pool
}

// newServerWithAssistant wires the chat assistant to a specific tool caller so a
// test can script the model's tool-use behavior.
func newServerWithAssistant(t *testing.T, caller llm.ToolCaller) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := bootstrapDB(t)
	h := handlersFor(pool, stores.NewService(mockLLM{}), recipes.NewGenerator(mockLLM{}), caller)
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	return srv, pool
}

func newServerNoLLM(t *testing.T) *httptest.Server {
	t.Helper()
	pool := bootstrapDB(t)
	var nilClient *llm.Client // unconfigured
	h := handlersFor(pool, stores.NewService(nilClient), recipes.NewGenerator(nilClient), nilClient)
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	return srv
}

// endTurnLLM is a no-op tool caller: it always ends the turn with empty text.
// Used by helpers whose tests do not exercise the chat assistant.
type endTurnLLM struct{}

func (endTurnLLM) CreateMessage(context.Context, string, []llm.Tool, []llm.Message) (llm.ToolResponse, error) {
	return llm.ToolResponse{StopReason: "end_turn", Content: []llm.ContentBlock{{Type: "text", Text: ""}}}, nil
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

// TestProfileWithDeletedUser reproduces the stale-JWT case: a token stays valid
// after its user row is gone (e.g. the DB was reset). Saving a profile must come
// back as 401 so the client drops the token, not 500.
func TestProfileWithDeletedUser(t *testing.T) {
	srv, pool := newServerWithPool(t)

	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/auth/register", "",
		map[string]string{"email": "ghost@example.com", "password": "supersecret"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, body %v", resp.StatusCode, out)
	}
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatal("expected token from register")
	}

	// The user vanishes while the token lives on.
	if _, err := pool.Exec(context.Background(), `DELETE FROM users`); err != nil {
		t.Fatalf("delete users: %v", err)
	}

	resp, out = doJSON(t, http.MethodPut, srv.URL+"/api/profile", token, map[string]any{
		"sex": "male", "height_cm": 180, "age": 30,
		"activity_level": "moderate", "goal": "maintain",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("put profile after user deleted = %d, want 401; body %v", resp.StatusCode, out)
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
