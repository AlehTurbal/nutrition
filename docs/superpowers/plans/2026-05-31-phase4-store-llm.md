# Phase 4 — Store Matching + Recipe Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Claude-API-powered store matching (needed ingredients ↔ pasted store assortment) and recipe generation to the nutrition app, degrading gracefully to HTTP 503 when no API key is configured.

**Architecture:** A thin `internal/llm` client wraps the Anthropic Messages API over `net/http` (no SDK) behind a `Completer` interface so handlers can be tested with a mock. Pure parse/match logic lives in `internal/stores` (matching) and is unit-tested without network or DB. New persistence (`store_matches`, `store_match_items`) follows the existing pgx `Store` pattern, scoped by `user_id`. New endpoints hang off the existing chi router under the JWT middleware. The frontend gets one "Магазин/Генерация" screen using existing TanStack Query patterns.

**Tech Stack:** Go 1.25 (via `golang:1.25-alpine` Docker — no local Go), pgx/v5, chi/v5, stdlib `net/http` + `encoding/json`; React + Vite + TS + Tailwind + TanStack Query.

---

## Conventions for every task

- **No local Go.** Run all Go commands through Docker. From repo root:
  - Unit: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/<pkg>/ -run <Name> -v`
  - Build+vet+all unit: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine sh -c 'go build ./... && go vet ./... && go test ./...'`
- **Integration tests** (httpapi) are skipped unless `NUTRITION_TEST_DATABASE_URL` is set; start Postgres first:
  ```bash
  docker compose -f deploy/docker-compose.yml up -d postgres   # wait until healthy
  docker run --rm --add-host=host.docker.internal:host-gateway \
    -e NUTRITION_TEST_DATABASE_URL='postgres://nutrition:nutrition@host.docker.internal:5432/nutrition?sslmode=disable' \
    -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/httpapi/ -v
  ```
  Tear down with `docker compose -f deploy/docker-compose.yml down`.
- Commit message footer line: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- Frontend commands run locally from `frontend/`: `npm run build`.

---

## File Structure

**Create:**
- `backend/internal/llm/client.go` — Anthropic Messages client + `Completer` interface + `Complete` (with prompt caching), `ErrNoAPIKey`.
- `backend/internal/llm/json.go` — `ExtractJSON` (pure: pull the first JSON object/array out of prose).
- `backend/internal/llm/json_test.go` — unit tests for `ExtractJSON`.
- `backend/internal/llm/client_test.go` — client tests using a mocked `http.RoundTripper`.
- `backend/internal/stores/match.go` — `Service` (depends on `llm.Completer`), prompt building, `parseMatches` (pure), domain types.
- `backend/internal/stores/match_test.go` — unit tests for prompt content + `parseMatches` via a mock completer.
- `backend/internal/stores/store.go` — pgx `Store`: persist/read `store_matches` + `store_match_items`.
- `backend/internal/recipes/generate.go` — `Generator` (depends on `llm.Completer`), `parseGenerated` (pure), `GeneratedRecipe` type, product-name matching helper.
- `backend/internal/recipes/generate_test.go` — unit tests for `parseGenerated` + product matching.
- `backend/internal/db/migrations/0004_store_matches.sql` — new tables.
- `backend/internal/httpapi/llmapi.go` — handlers: `storeMatch`, `getStoreMatches`, `generateRecipe`.
- `backend/internal/httpapi/llmapi_test.go` — integration tests with a mock completer.
- `frontend/src/api/llm.ts` — typed hooks: `useStoreMatch`, `useStoreMatches`, `useGenerateRecipe`.
- `frontend/src/features/store/StorePage.tsx` — screen: plan picker + store-text paste + match results.
- `frontend/src/features/store/GenerateRecipe.tsx` — recipe-generation panel.

**Modify:**
- `backend/internal/config/config.go` — add optional `AnthropicAPIKey`, `LLMModel` (default `claude-opus-4-8`).
- `backend/cmd/server/main.go` — build the llm client + stores/generator, wire into `Handlers`.
- `backend/internal/httpapi/api.go` — add fields to `Handlers`, register routes.
- `backend/internal/httpapi/integration_test.go` — add new tables to the DROP list; wire mock completer + new stores into the test `Handlers`.
- `frontend/src/api/types.ts` — wire types for matches + generated recipes.
- `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx` — add "Магазин" route + nav item.
- `frontend/src/features/recipes/RecipesPage.tsx` — link to recipe generation (optional entry point).
- `deploy/docker-compose.yml` — pass `ANTHROPIC_API_KEY` / `LLM_MODEL` env through to backend.

---

## Task 1: Config — optional API key + model

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add fields and loading (no test — config has no existing test file; covered indirectly by build)**

In `Config` struct add:
```go
	AnthropicAPIKey string
	LLMModel        string
```
In `Load`, after the JWTTTL block and before `return cfg, nil`:
```go
	cfg.AnthropicAPIKey = os.Getenv("ANTHROPIC_API_KEY")
	cfg.LLMModel = getenv("LLM_MODEL", "claude-opus-4-8")
```
(Neither is required; missing key is valid and disables LLM features.)

- [ ] **Step 2: Verify build**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "Phase 4: config for optional Anthropic API key + model

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: `llm.ExtractJSON` (pure JSON-from-prose)

**Files:**
- Create: `backend/internal/llm/json.go`
- Test: `backend/internal/llm/json_test.go`

- [ ] **Step 1: Write the failing test**

`backend/internal/llm/json_test.go`:
```go
package llm

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain object", `{"a":1}`, `{"a":1}`},
		{"object in prose", "Sure! Here:\n{\"a\":1}\nDone.", `{"a":1}`},
		{"array in prose", "result: [1,2,3] ok", `[1,2,3]`},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"nested braces", `prefix {"a":{"b":2}} suffix`, `{"a":{"b":2}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractJSON(c.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("ExtractJSON(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestExtractJSONNone(t *testing.T) {
	if _, err := ExtractJSON("no json here"); err == nil {
		t.Error("expected error when no JSON present")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/llm/ -run TestExtractJSON -v`
Expected: FAIL — `undefined: ExtractJSON`.

- [ ] **Step 3: Write minimal implementation**

`backend/internal/llm/json.go`:
```go
// Package llm is a thin client for the Anthropic Messages API plus helpers for
// coaxing structured JSON out of model responses.
package llm

import (
	"errors"
	"strings"
)

// ErrNoJSON is returned when no JSON value can be found in the text.
var ErrNoJSON = errors.New("no JSON found in response")

// ExtractJSON returns the first balanced JSON object or array found in s,
// tolerating surrounding prose or ```json fences.
func ExtractJSON(s string) (string, error) {
	start := -1
	var open, close byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start, open, close = i, '{', '}'
			break
		}
		if s[i] == '[' {
			start, open, close = i, '[', ']'
			break
		}
	}
	if start == -1 {
		return "", ErrNoJSON
	}

	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1]), nil
			}
		}
	}
	return "", ErrNoJSON
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/llm/ -run TestExtractJSON -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/llm/json.go backend/internal/llm/json_test.go
git commit -m "Phase 4: llm.ExtractJSON pulls JSON out of model prose

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: `llm` client + `Completer` interface

**Files:**
- Create: `backend/internal/llm/client.go`
- Test: `backend/internal/llm/client_test.go`

- [ ] **Step 1: Write the failing test**

`backend/internal/llm/client_test.go`:
```go
package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets a test stand in for http transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientComplete(t *testing.T) {
	var gotBody map[string]any
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-api-key") != "secret" {
			t.Errorf("missing api key header")
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		resp := `{"content":[{"type":"text","text":"hello world"}]}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(resp)),
			Header:     make(http.Header),
		}, nil
	})

	c := New("secret", "claude-opus-4-8")
	c.http = &http.Client{Transport: rt}

	out, err := c.Complete(context.Background(), "SYS", "USER")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != "hello world" {
		t.Errorf("got %q, want %q", out, "hello world")
	}
	if gotBody["model"] != "claude-opus-4-8" {
		t.Errorf("model = %v", gotBody["model"])
	}
	// System prompt must be sent as a cacheable block.
	sys, ok := gotBody["system"].([]any)
	if !ok || len(sys) == 0 {
		t.Fatalf("system not an array: %v", gotBody["system"])
	}
	block := sys[0].(map[string]any)
	if block["text"] != "SYS" {
		t.Errorf("system text = %v", block["text"])
	}
	if block["cache_control"] == nil {
		t.Error("expected cache_control on system block")
	}
}

func TestClientHTTPError(t *testing.T) {
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 429,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := New("secret", "claude-opus-4-8")
	c.http = &http.Client{Transport: rt}
	if _, err := c.Complete(context.Background(), "s", "u"); err == nil {
		t.Error("expected error on non-200")
	}
}

func TestNilClientErrNoAPIKey(t *testing.T) {
	var c *Client // nil => not configured
	if _, err := c.Complete(context.Background(), "s", "u"); err != ErrNoAPIKey {
		t.Errorf("got %v, want ErrNoAPIKey", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/llm/ -run TestClient -v`
Expected: FAIL — `undefined: New`, `undefined: Client`, `undefined: ErrNoAPIKey`.

- [ ] **Step 3: Write minimal implementation**

`backend/internal/llm/client.go`:
```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrNoAPIKey signals that LLM features are unconfigured (no API key). Handlers
// translate this to HTTP 503.
var ErrNoAPIKey = errors.New("LLM not configured: missing ANTHROPIC_API_KEY")

const (
	apiURL        = "https://api.anthropic.com/v1/messages"
	apiVersion    = "2023-06-01"
	maxTokens     = 4096
	requestTimout = 60 * time.Second
)

// Completer turns a system + user prompt into a single text completion. The llm
// package's *Client implements it; tests use a fake.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Client calls the Anthropic Messages API. A nil *Client is the "not
// configured" state and returns ErrNoAPIKey from Complete.
type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

// New returns a configured client. If apiKey is empty, New returns nil so the
// caller can treat "no key" uniformly via the nil receiver.
func New(apiKey, model string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: requestTimout},
	}
}

type sysBlock struct {
	Type         string         `json:"type"`
	Text         string         `json:"text"`
	CacheControl map[string]any `json:"cache_control,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type requestBody struct {
	Model     string     `json:"model"`
	MaxTokens int        `json:"max_tokens"`
	System    []sysBlock `json:"system"`
	Messages  []message  `json:"messages"`
}

type responseBody struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends one system+user turn and returns the concatenated text. The
// system prompt is sent as a cacheable block (ephemeral prompt caching).
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	if c == nil {
		return "", ErrNoAPIKey
	}

	body, err := json.Marshal(requestBody{
		Model:     c.model,
		MaxTokens: maxTokens,
		System: []sysBlock{{
			Type:         "text",
			Text:         system,
			CacheControl: map[string]any{"type": "ephemeral"},
		}},
		Messages: []message{{Role: "user", Content: user}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed responseBody
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode anthropic response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := "non-200 from anthropic"
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return "", fmt.Errorf("anthropic %d: %s", resp.StatusCode, msg)
	}

	var out string
	for _, b := range parsed.Content {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/llm/ -v`
Expected: PASS (client + json tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/llm/client.go backend/internal/llm/client_test.go
git commit -m "Phase 4: Anthropic Messages client with prompt caching + nil-safe ErrNoAPIKey

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: `stores` matching service (pure parse + prompt)

**Files:**
- Create: `backend/internal/stores/match.go`
- Test: `backend/internal/stores/match_test.go`

- [ ] **Step 1: Write the failing test**

`backend/internal/stores/match_test.go`:
```go
package stores

import (
	"context"
	"strings"
	"testing"
)

type fakeCompleter struct {
	gotSystem, gotUser string
	reply              string
	err                error
}

func (f *fakeCompleter) Complete(_ context.Context, system, user string) (string, error) {
	f.gotSystem, f.gotUser = system, user
	return f.reply, f.err
}

func TestMatchBuildsPromptAndParses(t *testing.T) {
	fc := &fakeCompleter{reply: `Here:
[{"needed":"Курица","matched":"Chicken breast 1kg","found":true},
 {"needed":"Соль","matched":"","found":false}]`}
	svc := NewService(fc)

	needed := []Needed{{Name: "Курица", Grams: 300}, {Name: "Соль", Grams: 5}}
	got, err := svc.Match(context.Background(), needed, "Chicken breast 1kg - 5.99\nMilk 1L - 1.20")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	// Prompt must contain both needed names and the store text.
	if !strings.Contains(fc.gotUser, "Курица") || !strings.Contains(fc.gotUser, "Chicken breast 1kg") {
		t.Errorf("user prompt missing inputs: %q", fc.gotUser)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].Needed != "Курица" || got[0].Matched != "Chicken breast 1kg" || !got[0].Found {
		t.Errorf("result[0] wrong: %+v", got[0])
	}
	if got[1].Found {
		t.Errorf("result[1] should be not found: %+v", got[1])
	}
}

func TestMatchEmptyNeeded(t *testing.T) {
	svc := NewService(&fakeCompleter{})
	got, err := svc.Match(context.Background(), nil, "anything")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result for no needed items, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/stores/ -run TestMatch -v`
Expected: FAIL — `undefined: NewService`, `undefined: Needed`.

- [ ] **Step 3: Write minimal implementation**

`backend/internal/stores/match.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/stores/ -run TestMatch -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/stores/match.go backend/internal/stores/match_test.go
git commit -m "Phase 4: stores.Service matches needed ingredients to store text via LLM

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Migration 0004 + stores persistence

**Files:**
- Create: `backend/internal/db/migrations/0004_store_matches.sql`
- Create: `backend/internal/stores/store.go`

- [ ] **Step 1: Write the migration**

`backend/internal/db/migrations/0004_store_matches.sql`:
```sql
CREATE TABLE store_matches (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    meal_plan_id BIGINT NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_store_matches_user_plan ON store_matches (user_id, meal_plan_id, created_at DESC);

CREATE TABLE store_match_items (
    id             BIGSERIAL PRIMARY KEY,
    store_match_id BIGINT NOT NULL REFERENCES store_matches(id) ON DELETE CASCADE,
    needed_name    TEXT NOT NULL,
    needed_grams   DOUBLE PRECISION NOT NULL,
    matched_text   TEXT NOT NULL DEFAULT '',
    found          BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_store_match_items_match ON store_match_items (store_match_id);
```

- [ ] **Step 2: Write the persistence store (no separate unit test — exercised by the httpapi integration test in Task 8)**

`backend/internal/stores/store.go`:
```go
package stores

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when no saved match exists for a plan.
var ErrNotFound = errors.New("store match not found")

// SavedMatch is a persisted matching run for a plan.
type SavedMatch struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	MealPlanID int64     `json:"meal_plan_id"`
	CreatedAt  time.Time `json:"created_at"`
	Items      []Match   `json:"items"`
}

// Store persists store-matching proposals.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Save writes a matching run for a plan owned by the user. The plan must belong
// to the user (verified by the FK + explicit ownership check).
func (s *Store) Save(ctx context.Context, userID, planID int64, items []Match) (SavedMatch, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SavedMatch{}, err
	}
	defer tx.Rollback(ctx)

	var owns bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM meal_plans WHERE id=$1 AND user_id=$2)`,
		planID, userID).Scan(&owns); err != nil {
		return SavedMatch{}, fmt.Errorf("verify plan: %w", err)
	}
	if !owns {
		return SavedMatch{}, ErrNotFound
	}

	var m SavedMatch
	err = tx.QueryRow(ctx,
		`INSERT INTO store_matches (user_id, meal_plan_id) VALUES ($1,$2)
		 RETURNING id, user_id, meal_plan_id, created_at`,
		userID, planID,
	).Scan(&m.ID, &m.UserID, &m.MealPlanID, &m.CreatedAt)
	if err != nil {
		return SavedMatch{}, fmt.Errorf("insert store_match: %w", err)
	}

	for _, it := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO store_match_items (store_match_id, needed_name, needed_grams, matched_text, found)
			 VALUES ($1,$2,$3,$4,$5)`,
			m.ID, it.Needed, 0.0, it.Matched, it.Found); err != nil {
			return SavedMatch{}, fmt.Errorf("insert item: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedMatch{}, err
	}
	m.Items = items
	return m, nil
}

// Latest returns the most recent saved match for a plan owned by the user.
func (s *Store) Latest(ctx context.Context, userID, planID int64) (SavedMatch, error) {
	var m SavedMatch
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, meal_plan_id, created_at FROM store_matches
		 WHERE user_id=$1 AND meal_plan_id=$2 ORDER BY created_at DESC, id DESC LIMIT 1`,
		userID, planID,
	).Scan(&m.ID, &m.UserID, &m.MealPlanID, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedMatch{}, ErrNotFound
	}
	if err != nil {
		return SavedMatch{}, fmt.Errorf("latest match: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT needed_name, matched_text, found FROM store_match_items
		 WHERE store_match_id=$1 ORDER BY id`, m.ID)
	if err != nil {
		return SavedMatch{}, fmt.Errorf("load items: %w", err)
	}
	defer rows.Close()
	m.Items = []Match{}
	for rows.Next() {
		var it Match
		if err := rows.Scan(&it.Needed, &it.Matched, &it.Found); err != nil {
			return SavedMatch{}, fmt.Errorf("scan item: %w", err)
		}
		m.Items = append(m.Items, it)
	}
	return m, rows.Err()
}
```

- [ ] **Step 3: Verify build + vet**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine sh -c 'go build ./... && go vet ./...'`
Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/db/migrations/0004_store_matches.sql backend/internal/stores/store.go
git commit -m "Phase 4: store_matches schema + persistence (user-scoped)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Recipe generation (pure parse + product matching)

**Files:**
- Create: `backend/internal/recipes/generate.go`
- Test: `backend/internal/recipes/generate_test.go`

- [ ] **Step 1: Write the failing test**

`backend/internal/recipes/generate_test.go`:
```go
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
			{Name: "молоко", Grams: 200},        // case-insensitive match -> id 1
			{Name: "Овсяные хлопья", Grams: 60}, // exact -> id 2
			{Name: "Соль", Grams: 2},            // unmatched
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/recipes/ -run 'TestGenerate|TestMatchIngredients' -v`
Expected: FAIL — `undefined: NewGenerator`, `undefined: GeneratedRecipe`, etc.

- [ ] **Step 3: Write minimal implementation**

`backend/internal/recipes/generate.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/recipes/ -v`
Expected: PASS (existing recipe tests + new ones).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/recipes/generate.go backend/internal/recipes/generate_test.go
git commit -m "Phase 4: LLM recipe generation + name->product matching (pure)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: HTTP handlers + routes + wiring

**Files:**
- Create: `backend/internal/httpapi/llmapi.go`
- Modify: `backend/internal/httpapi/api.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add handlers**

`backend/internal/httpapi/llmapi.go`:
```go
package httpapi

import (
	"errors"
	"net/http"

	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/llm"
	"github.com/alehturbal/nutrition/backend/internal/mealplans"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/stores"
)

// llm503 writes a 503 when the LLM is unconfigured; returns true if it handled err.
func llm503(w http.ResponseWriter, err error) bool {
	if errors.Is(err, llm.ErrNoAPIKey) {
		writeError(w, http.StatusServiceUnavailable, "LLM-функции недоступны: не задан ANTHROPIC_API_KEY")
		return true
	}
	return false
}

type storeMatchRequest struct {
	PlanID    int64  `json:"plan_id"`
	StoreText string `json:"store_text"`
}

func (h *Handlers) storeMatch(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req storeMatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreText == "" {
		writeError(w, http.StatusBadRequest, "store_text is required")
		return
	}

	sl, err := h.MealPlans.ShoppingList(r.Context(), userID, req.PlanID)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load plan needs")
		return
	}

	needed := make([]stores.Needed, len(sl.Items))
	for i, it := range sl.Items {
		needed[i] = stores.Needed{Name: it.ProductName, Grams: it.Grams}
	}

	matches, err := h.StoreMatcher.Match(r.Context(), needed, req.StoreText)
	if llm503(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "LLM matching failed")
		return
	}

	saved, err := h.StoreStore.Save(r.Context(), userID, req.PlanID, matches)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save match")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *Handlers) getStoreMatches(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	planID, ok := pathIDParam(w, r, "planID")
	if !ok {
		return
	}
	saved, err := h.StoreStore.Latest(r.Context(), userID, planID)
	if errors.Is(err, stores.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no saved match for this plan")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load match")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type generateRequest struct {
	Description string   `json:"description"`
	MealTypes   []string `json:"meal_types"`
	Servings    int      `json:"servings"`
}

func (h *Handlers) generateRecipe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req generateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "description is required")
		return
	}

	gen, err := h.RecipeGen.Generate(r.Context(), req.Description, req.MealTypes, req.Servings)
	if llm503(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "LLM generation failed")
		return
	}

	owned, err := h.Products.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load products")
		return
	}
	pl := make([]recipes.Product, len(owned))
	for i, p := range owned {
		pl[i] = recipes.Product{ID: p.ID, Name: p.Name}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recipe":      gen,
		"ingredients": recipes.MatchIngredientsToProducts(gen, pl),
	})
}
```

- [ ] **Step 2: Wire into Handlers + routes**

In `backend/internal/httpapi/api.go`, add imports:
```go
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/stores"
```
(Note: `recipes` is already imported; only add `stores`.) Add an interface near the top of the file for the matcher/generator so tests can inject fakes:
```go
// StoreMatcher matches needed ingredients to store text (stores.Service in prod).
type StoreMatcher interface {
	Match(ctx context.Context, needed []stores.Needed, storeText string) ([]stores.Match, error)
}

// RecipeGenerator produces a recipe draft (recipes.Generator in prod).
type RecipeGenerator interface {
	Generate(ctx context.Context, description string, slots []string, servings int) (recipes.GeneratedRecipe, error)
}
```
Add a `context` import if not present. Extend the `Handlers` struct:
```go
	StoreMatcher StoreMatcher
	StoreStore   *stores.Store
	RecipeGen    RecipeGenerator
```
Inside `r.Group(func(r chi.Router){ ... })` (the authed group), register routes:
```go
			r.Post("/stores/match", h.storeMatch)
			r.Get("/stores/matches/{planID}", h.getStoreMatches)
			r.Post("/recipes/generate", h.generateRecipe)
```
Place `r.Post("/recipes/generate", ...)` OUTSIDE the existing `r.Route("/recipes", ...)` sub-router (register it on the parent group as shown) to avoid path conflicts with `/{id}`.

In `backend/cmd/server/main.go`, add imports `internal/llm` and `internal/stores`, then before constructing `Handlers`:
```go
	llmClient := llm.New(cfg.AnthropicAPIKey, cfg.LLMModel)
```
and add to the `Handlers` literal:
```go
		StoreMatcher: stores.NewService(llmClient),
		StoreStore:   stores.NewStore(pool),
		RecipeGen:    recipes.NewGenerator(llmClient),
```
(`recipes` is already imported in main.go.) Note: passing a nil `*llm.Client` is intentional — `Complete` returns `ErrNoAPIKey` and handlers map it to 503.

- [ ] **Step 3: Verify build + vet + all unit tests**

Run: `docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine sh -c 'go build ./... && go vet ./... && go test ./...'`
Expected: all packages `ok` / `no test files`; no build or vet errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/httpapi/llmapi.go backend/internal/httpapi/api.go backend/cmd/server/main.go
git commit -m "Phase 4: HTTP endpoints for store match + recipe generation (503 when unconfigured)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 8: httpapi integration tests (mock LLM)

**Files:**
- Create: `backend/internal/httpapi/llmapi_test.go`
- Modify: `backend/internal/httpapi/integration_test.go`

- [ ] **Step 1: Update the test harness**

In `backend/internal/httpapi/integration_test.go`:
- Add the new tables to the DROP statement (front of the list):
  ```go
  `DROP TABLE IF EXISTS store_match_items, store_matches, meal_plan_items, meal_plans, recipe_ingredients, recipes, products, weight_entries, profiles, users, schema_migrations CASCADE`
  ```
- Add imports `"github.com/alehturbal/nutrition/backend/internal/stores"` and `"context"` if missing.
- In the `newServer` helper, the `Handlers` literal must set the new fields. Because tests must not hit the network, define a mock completer in the test file (Task 8 Step 2) and inject it:
  ```go
  Products:     products.NewStore(pool),
  // ...existing fields...
  StoreMatcher: stores.NewService(mockLLM{}),
  StoreStore:   stores.NewStore(pool),
  RecipeGen:    recipes.NewGenerator(mockLLM{}),
  ```
  (Add `recipes` import if not present.) Keep this in sync with the production literal in main.go.

- [ ] **Step 2: Write the integration tests**

`backend/internal/httpapi/llmapi_test.go`:
```go
package httpapi_test

import (
	"context"
	"net/http"
	"testing"
)

// mockLLM is a deterministic Completer for tests (no network).
type mockLLM struct{}

func (mockLLM) Complete(_ context.Context, system, _ string) (string, error) {
	// Distinguish the two prompts by a marker in the system text.
	if len(system) > 0 && system[0] == 'Y' { // recipe generation prompt starts "You are a cooking assistant"
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

	// Latest returns the saved proposal.
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
	// Курица matches the owned product (non-nil product_id); Соль does not.
	if ings[0].(map[string]any)["product_id"] == nil {
		t.Error("expected Курица to match a product")
	}
	if ings[1].(map[string]any)["product_id"] != nil {
		t.Error("expected Соль to be unmatched")
	}
}
```
Note: the `mockLLM.Complete` discriminator relies on the recipe-generation system prompt beginning with "You..." and the store-match prompt beginning with "You match...". Both begin with "You", so instead check for a more specific marker: change the condition to `strings.Contains(system, "cooking assistant")`. Update the test to `import "strings"` and use:
```go
	if strings.Contains(system, "cooking assistant") {
```

- [ ] **Step 3: Run integration tests against Postgres**

```bash
docker compose -f deploy/docker-compose.yml up -d postgres   # wait until healthy
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e NUTRITION_TEST_DATABASE_URL='postgres://nutrition:nutrition@host.docker.internal:5432/nutrition?sslmode=disable' \
  -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/httpapi/ -v
docker compose -f deploy/docker-compose.yml down
```
Expected: all tests PASS, including `TestStoreMatchFlow` and `TestGenerateRecipeFlow`.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/httpapi/llmapi_test.go backend/internal/httpapi/integration_test.go
git commit -m "Phase 4: integration tests for store match + recipe generation (mock LLM)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 9: 503 graceful-degradation test

**Files:**
- Modify: `backend/internal/httpapi/llmapi_test.go`

- [ ] **Step 1: Add a test using a real nil llm.Client (returns ErrNoAPIKey)**

Append to `backend/internal/httpapi/llmapi_test.go`:
```go
func TestStoreMatchNoAPIKey(t *testing.T) {
	srv := newServerNoLLM(t)
	token := registerUser(t, srv.URL, "nokey@example.com")
	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "P", "start_date": "2026-06-01", "end_date": "2026-06-02",
	})
	pid := int64(out["id"].(float64))

	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/api/stores/match", token, map[string]any{
		"plan_id": pid, "store_text": "anything",
	})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without API key, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Add the `newServerNoLLM` helper to `integration_test.go`**

This mirrors `newServer` but injects a nil `*llm.Client` (so `Complete` returns `ErrNoAPIKey`). Add to `backend/internal/httpapi/integration_test.go` (add imports `"github.com/alehturbal/nutrition/backend/internal/llm"`):
```go
func newServerNoLLM(t *testing.T) *httptest.Server {
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
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS store_match_items, store_matches, meal_plan_items, meal_plans, recipe_ingredients, recipes, products, weight_entries, profiles, users, schema_migrations CASCADE`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := users.NewStore(pool)
	tokens := auth.NewManager("test-secret", time.Hour)
	var nilClient *llm.Client // unconfigured
	h := &httpapi.Handlers{
		Auth:         auth.NewService(store, tokens),
		Users:        store,
		Tokens:       tokens,
		Products:     products.NewStore(pool),
		Recipes:      recipes.NewStore(pool),
		MealPlans:    mealplans.NewStore(pool),
		StoreMatcher: stores.NewService(nilClient),
		StoreStore:   stores.NewStore(pool),
		RecipeGen:    recipes.NewGenerator(nilClient),
	}
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	return srv
}
```

- [ ] **Step 3: Run integration tests**

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e NUTRITION_TEST_DATABASE_URL='postgres://nutrition:nutrition@host.docker.internal:5432/nutrition?sslmode=disable' \
  -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/httpapi/ -run 'TestStoreMatchNoAPIKey|TestStoreMatchFlow|TestGenerateRecipeFlow' -v
docker compose -f deploy/docker-compose.yml down
```
Expected: all three PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/httpapi/llmapi_test.go backend/internal/httpapi/integration_test.go
git commit -m "Phase 4: test 503 graceful degradation when API key absent

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 10: docker-compose env passthrough

**Files:**
- Modify: `deploy/docker-compose.yml`

- [ ] **Step 1: Pass the key + model to the backend**

In the `backend.environment` block add:
```yaml
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
      LLM_MODEL: ${LLM_MODEL:-claude-opus-4-8}
```
(The `:-` default keeps the stack working when the var is unset — features simply return 503.)

- [ ] **Step 2: Verify compose config parses**

Run: `docker compose -f deploy/docker-compose.yml config >/dev/null && echo OK`
Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add deploy/docker-compose.yml
git commit -m "Phase 4: pass ANTHROPIC_API_KEY/LLM_MODEL to backend in compose

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 11: Frontend — API types + hooks

**Files:**
- Modify: `frontend/src/api/types.ts`
- Create: `frontend/src/api/llm.ts`

- [ ] **Step 1: Add wire types**

Append to `frontend/src/api/types.ts`:
```ts
export interface StoreMatchItem {
  needed: string;
  matched: string;
  found: boolean;
}

export interface SavedStoreMatch {
  id: number;
  user_id: number;
  meal_plan_id: number;
  created_at: string;
  items: StoreMatchItem[];
}

export interface GeneratedIngredient {
  name: string;
  grams: number;
}

export interface GeneratedRecipe {
  name: string;
  servings: number;
  instructions: string;
  meal_types: MealSlot[];
  ingredients: GeneratedIngredient[];
}

export interface MatchedIngredient {
  name: string;
  grams: number;
  product_id: number | null;
}

export interface GenerateResponse {
  recipe: GeneratedRecipe;
  ingredients: MatchedIngredient[];
}
```

- [ ] **Step 2: Add hooks**

`frontend/src/api/llm.ts`:
```ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./client";
import type {
  GenerateResponse,
  SavedStoreMatch,
} from "./types";

export function useStoreMatch() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { plan_id: number; store_text: string }) =>
      api.post<SavedStoreMatch>("/api/stores/match", input),
    onSuccess: (_d, vars) =>
      qc.invalidateQueries({ queryKey: ["store-match", vars.plan_id] }),
  });
}

export function useStoreMatches(planId: number | null) {
  return useQuery({
    queryKey: ["store-match", planId],
    enabled: planId != null,
    queryFn: async () => {
      try {
        return await api.get<SavedStoreMatch>(`/api/stores/matches/${planId}`);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
  });
}

export function useGenerateRecipe() {
  return useMutation({
    mutationFn: (input: {
      description: string;
      meal_types: string[];
      servings: number;
    }) => api.post<GenerateResponse>("/api/recipes/generate", input),
  });
}
```

- [ ] **Step 3: Verify build**

Run (from `frontend/`): `npm run build`
Expected: `tsc -b` clean, vite build succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/api/types.ts frontend/src/api/llm.ts
git commit -m "Phase 4: frontend API types + hooks for store match + recipe generation

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 12: Frontend — Store screen + Generate panel + nav

**Files:**
- Create: `frontend/src/features/store/StorePage.tsx`
- Create: `frontend/src/features/store/GenerateRecipe.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/Layout.tsx`

- [ ] **Step 1: Recipe generation panel**

`frontend/src/features/store/GenerateRecipe.tsx`:
```tsx
import { useState } from "react";
import { useGenerateRecipe } from "../../api/llm";
import { Button, Card, ErrorBox, Field } from "../../components/ui";
import { fmt, mealSlots, slotLabel } from "../../lib/labels";

export default function GenerateRecipe() {
  const gen = useGenerateRecipe();
  const [description, setDescription] = useState("");
  const [servings, setServings] = useState(1);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    gen.mutate({ description: description.trim(), meal_types: [], servings });
  };

  return (
    <Card title="Генерация рецепта (LLM)">
      <form onSubmit={submit} className="space-y-3">
        <Field
          label="Опишите блюдо"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="например: лёгкий салат с курицей"
          required
        />
        <Field
          label="Порций"
          type="number"
          min={1}
          value={servings}
          onChange={(e) => setServings(Number(e.target.value))}
        />
        {gen.isError && <ErrorBox error={gen.error} />}
        <Button type="submit" disabled={gen.isPending}>
          {gen.isPending ? "Генерация…" : "Сгенерировать"}
        </Button>
      </form>

      {gen.data && (
        <div className="mt-4 space-y-2 border-t border-slate-100 pt-4">
          <h3 className="font-semibold text-slate-700">{gen.data.recipe.name}</h3>
          <div className="flex flex-wrap gap-1">
            {gen.data.recipe.meal_types.map((m) => (
              <span key={m} className="rounded-full bg-brand-50 px-2 py-0.5 text-xs text-brand-700">
                {slotLabel(m)}
              </span>
            ))}
          </div>
          <p className="text-sm text-slate-500">{gen.data.recipe.instructions}</p>
          <ul className="text-sm">
            {gen.data.ingredients.map((ing, i) => (
              <li key={i} className="flex justify-between border-b border-slate-50 py-1">
                <span>
                  {ing.name}{" "}
                  {ing.product_id == null && (
                    <span className="text-amber-600" title="Нет такого продукта">
                      (создать продукт)
                    </span>
                  )}
                </span>
                <span className="text-slate-500">{fmt(ing.grams)} г</span>
              </li>
            ))}
          </ul>
          <p className="text-xs text-slate-400">
            Чтобы сохранить рецепт, создайте недостающие продукты на вкладке «Продукты»,
            затем добавьте рецепт вручную. (Автосохранение появится позже.)
          </p>
        </div>
      )}
      <span className="sr-only">{mealSlots.length}</span>
    </Card>
  );
}
```
(The `mealSlots.length` reference keeps the import used without changing layout; remove if you wire meal-type selection.)

- [ ] **Step 2: Store matching screen**

`frontend/src/features/store/StorePage.tsx`:
```tsx
import { useEffect, useState } from "react";
import { usePlans } from "../../api/plans";
import { useStoreMatch, useStoreMatches } from "../../api/llm";
import { Button, Card, Empty, ErrorBox, Spinner } from "../../components/ui";
import { shortDate } from "../../lib/dates";
import GenerateRecipe from "./GenerateRecipe";

export default function StorePage() {
  const plans = usePlans();
  const [planId, setPlanId] = useState<number | null>(null);
  const [storeText, setStoreText] = useState("");
  const match = useStoreMatch();
  const saved = useStoreMatches(planId);

  useEffect(() => {
    if (planId == null && plans.data && plans.data.length > 0) {
      setPlanId(plans.data[0].id);
    }
  }, [plans.data, planId]);

  const result = match.data ?? saved.data ?? null;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-800">Магазин</h1>

      {plans.isLoading ? (
        <Spinner />
      ) : !plans.data || plans.data.length === 0 ? (
        <Card><Empty>Сначала создайте план питания</Empty></Card>
      ) : (
        <Card title="Подбор товаров под план">
          <div className="space-y-3">
            <select
              value={planId ?? ""}
              onChange={(e) => setPlanId(Number(e.target.value))}
              className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm"
            >
              {plans.data.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name || "Без названия"} ({shortDate(p.start_date)}–{shortDate(p.end_date)})
                </option>
              ))}
            </select>

            <label className="block text-sm">
              <span className="mb-1 block font-medium text-slate-600">
                Вставьте ассортимент магазина (любой язык)
              </span>
              <textarea
                value={storeText}
                onChange={(e) => setStoreText(e.target.value)}
                rows={6}
                className="w-full rounded-lg border border-slate-300 px-3 py-1.5 text-sm outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                placeholder="Скопируйте список товаров со страницы магазина…"
              />
            </label>

            {match.isError && <ErrorBox error={match.error} />}
            <Button
              disabled={planId == null || storeText.trim() === "" || match.isPending}
              onClick={() => planId != null && match.mutate({ plan_id: planId, store_text: storeText })}
            >
              {match.isPending ? "Подбор…" : "Подобрать"}
            </Button>
          </div>

          {result && (
            <div className="mt-4 border-t border-slate-100 pt-4">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase text-slate-400">
                    <th className="py-1">Нужно</th>
                    <th className="py-1">Найдено в магазине</th>
                  </tr>
                </thead>
                <tbody>
                  {result.items.map((it, i) => (
                    <tr key={i} className="border-t border-slate-50">
                      <td className="py-1.5 font-medium text-slate-700">{it.needed}</td>
                      <td className="py-1.5">
                        {it.found ? (
                          it.matched
                        ) : (
                          <span className="text-amber-600">не найдено</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}

      <GenerateRecipe />
    </div>
  );
}
```

- [ ] **Step 3: Add route + nav item**

In `frontend/src/App.tsx`, import and add a route:
```tsx
import StorePage from "./features/store/StorePage";
```
```tsx
        <Route path="/store" element={<StorePage />} />
```
In `frontend/src/components/Layout.tsx`, add to the `nav` array (after the shopping entry):
```tsx
  { to: "/store", label: "Магазин" },
```

- [ ] **Step 4: Verify build**

Run (from `frontend/`): `npm run build`
Expected: `tsc -b` clean, vite build succeeds.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/store frontend/src/App.tsx frontend/src/components/Layout.tsx
git commit -m "Phase 4: frontend store-matching screen + recipe generation panel + nav

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 13: Final verification + docs

**Files:**
- Modify: `AGENTS.md` / `CLAUDE.md` (note Phase 4 done, new endpoints), if they enumerate phases.

- [ ] **Step 1: Full backend suite (unit + integration)**

```bash
docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine sh -c 'go build ./... && go vet ./... && go test ./...'
docker compose -f deploy/docker-compose.yml up -d postgres
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e NUTRITION_TEST_DATABASE_URL='postgres://nutrition:nutrition@host.docker.internal:5432/nutrition?sslmode=disable' \
  -v "$PWD/backend":/app -w /app golang:1.25-alpine go test ./internal/httpapi/ -v
docker compose -f deploy/docker-compose.yml down
```
Expected: everything green.

- [ ] **Step 2: Full frontend build**

Run (from `frontend/`): `npm run build`
Expected: clean.

- [ ] **Step 3: Update phase status in AGENTS.md/CLAUDE.md if present**

Mark Phase 4 done; list new endpoints (`POST /api/stores/match`, `GET /api/stores/matches/{planID}`, `POST /api/recipes/generate`) and the `ANTHROPIC_API_KEY` / `LLM_MODEL` env vars.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md CLAUDE.md
git commit -m "Phase 4: docs — mark store+LLM done, list new endpoints/env

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-review notes (addressed)

- **Spec coverage:** store matching (Tasks 4,5,7,8,12), recipe generation (Tasks 6,7,8,12), `internal/llm` + prompt caching (Tasks 2,3), graceful 503 (Tasks 1,3,7,9), data model 0004 (Task 5), endpoints (Task 7), frontend (Tasks 11,12), config/compose (Tasks 1,10). БЖУ pre-fill intentionally excluded per spec.
- **Type consistency:** `Completer.Complete(ctx, system, user)` used identically across llm/stores/recipes/httpapi; `stores.Needed{Name,Grams}`, `stores.Match{Needed,Matched,Found}`, `recipes.GeneratedRecipe`/`GeneratedIngredient`/`MatchedIngredient`, `recipes.Product{ID,Name}` consistent between definition (Tasks 4,6) and use (Tasks 7,8). `pathIDParam` already exists in mealplans.go and is reused for `{planID}`.
- **Cross-package cycle avoided:** `recipes.Product` is a local minimal type, so `recipes` does not import `products`; the handler converts `products.Product` → `recipes.Product`.
- **Routing caveat:** `/recipes/generate` is registered on the authed parent group, not inside `r.Route("/recipes")`, to avoid colliding with `/{id}`.
- **mockLLM discriminator:** uses `strings.Contains(system, "cooking assistant")` (corrected in Task 8 Step 2) to pick recipe vs. match reply.
