# Phase 4 — store + LLM — design

Adds Claude-API-powered features to the nutrition app: matching needed
ingredients against a store's assortment, and generating recipes. БЖУ pre-fill
is explicitly **out of scope** for this phase (the frontend's "fill via LLM"
button stays disabled).

## Scope
- **Store matching** (primary): from a meal plan's needed ingredients, the LLM
  finds matching products in a store and proposes a list ("needed ↔ available",
  not a catalog import).
- **Recipe generation**: from a free-text description, the LLM returns a
  structured recipe draft (name, instructions, meal tags, ingredients as
  name+grams) for the user to confirm.

## How a store's assortment is obtained
The user **pastes the store's assortment text** into a field (decided over
fetch+scrape and headless-browser, which break on SPAs / bot protection). This
works with any store in any language and needs no scraping infrastructure. The
raw pasted text is sent to the LLM together with the needed-ingredient list.

## `internal/llm` — Claude API client
Thin wrapper over the Anthropic Messages API.
- Model from env (`LLM_MODEL`), default `claude-opus-4-8`.
- **Prompt caching** on the stable system prompt; the per-request user content
  (ingredient list, store text, recipe description) is the variable part.
- API key from `ANTHROPIC_API_KEY` (env / k8s Secret). **Optional**: if unset,
  the client is `nil` and LLM endpoints return **503** with a clear message;
  the rest of the app is unaffected (graceful degradation).
- Responses are required to be JSON; parsing is tolerant of surrounding prose
  (extract the JSON object/array, then unmarshal).
- The client is defined behind an interface so handlers/services can be tested
  with a mock; no real network calls in tests/CI.

## Store matching — `internal/stores`
Flow: user opens "Подбор", picks a plan → sees the plan's **needed ingredients**
(reused from the existing shopping-list aggregation) → pastes store assortment
text (any language) → backend sends `{needed ingredients} + {raw store text}` to
the LLM → LLM returns, per needed ingredient, the matched store product(s)
(store-language name, optional price/size) or "not found". The result is a
**proposal** shown as a list and cached in `store_matches` + `store_match_items`
(scoped by `user_id`, keyed by plan).

## Recipe generation
User describes the desired dish (optional meal slots, servings). The LLM returns
a structured recipe: name, instructions, meal tags, ingredients as **name +
grams**. Because our `recipes` rows reference `products.id`, a matching step maps
each ingredient name to one of the user's products (case-insensitive); unmatched
ingredients are surfaced as "create this product first". The recipe is saved
only after the user confirms/creates the needed products — preserving the
invariant "every ingredient references an existing product owned by the user".
`POST /api/recipes/generate` returns the draft and does **not** save it.

## Data model (migration `0004`)
- `store_matches(id, user_id, meal_plan_id, created_at)`.
- `store_match_items(id, store_match_id, needed_name, needed_grams,
  matched_text, found bool)`.
Both carry/relate to `user_id`; cascade on user/plan delete.

## HTTP (all behind JWT middleware)
- `POST /api/stores/match` — body: plan id + store text → matching proposal
  (also persisted).
- `GET /api/stores/matches/{planId}` — latest persisted match for a plan.
- `POST /api/recipes/generate` — body: description (+ optional slots/servings) →
  recipe draft + per-ingredient product-match status. Does not persist.

## Config
`config.go` gains optional `AnthropicAPIKey` and `LLMModel` (default
`claude-opus-4-8`). Missing key ⇒ LLM features disabled (503), app otherwise
runs.

## Testing
- `internal/llm`: client tested with a **swappable HTTP transport** (mocked
  Anthropic responses); JSON extraction/parse unit-tested.
- `internal/stores`: matching/parse logic as **pure functions** over a mock LLM
  interface; no network.
- `httpapi`: integration — 503 when key absent; happy path with a mock LLM
  injected into `Handlers`.
- No real Anthropic network calls in CI.

## Invariants preserved (per CLAUDE.md)
LLM client behind an interface (mockable); parse/match logic pure; all new
tables/queries scoped by `user_id`; recipe generation respects "ingredient →
user-owned product"; БЖУ keeps its `complete` flag.

## Out of scope
БЖУ pre-fill (deferred); chat assistant (Phase 5); Kubernetes (Phase 6).
