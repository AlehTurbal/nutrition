# Phase 5 — chat assistant — design

A persistent right-side chat panel lets the user operate the app in natural
language. The assistant reads data immediately and proposes mutations for the
user to confirm. Built on the Phase 4 `llm.Client` with Claude tool-use.

## Decisions (from brainstorming)
- **Tool set v1:** read + create products/recipes only.
- **Propose → confirm:** preview without persistence. A mutating tool call
  becomes a *proposal* returned to the frontend; the user clicks "Применить",
  which calls the existing CRUD endpoint. No `chat_pending_actions` table.
- **History:** multiple threads per user, persisted.

## `internal/assistant`
A tool-use loop over `llm.Client` (Phase 4). System prompt describes the
assistant and its tools. Two tool classes:
- **Read tools (executed immediately):** `list_products`, `list_recipes`,
  `get_targets`. Results are fed back to the model as `tool_result`; the loop
  continues until the model returns a final text answer. A max-iteration cap
  guards against loops.
- **Mutating tools (NOT executed):** `propose_product`, `propose_recipe`. When
  the model calls one, the backend does **not** execute it; it converts the tool
  input into a *proposal* (`type` + `payload` shaped for `POST /api/products` or
  `POST /api/recipes`) and returns it to the frontend alongside the assistant's
  text.

`llm` gains a method that takes a full message history + tool definitions and
returns the raw response (stop_reason, content blocks incl. tool_use), beside
the existing `Complete`. It sits behind an interface and is mocked in tests; no
real network in tests. Parsing of tool-use blocks into proposals / read-tool
dispatch is pure and unit-tested.

## Data model (migration `0005`)
- `chat_threads(id, user_id, title, created_at)` — multiple threads per user.
- `chat_messages(id, thread_id, role, content, created_at)` — `role` is
  `user` | `assistant`; `content` is the visible text. The tool-use exchange is
  not persisted (only the visible dialog). User scoping flows through the thread
  (`chat_threads.user_id`); every query joins/filters on it.
- No `chat_pending_actions` table (preview-without-persistence).

## HTTP (all behind JWT middleware)
- `GET /api/chat/threads` — list user's threads.
- `POST /api/chat/threads` — create a thread (optional title).
- `DELETE /api/chat/threads/{id}` — delete a thread (cascades messages).
- `GET /api/chat/threads/{id}/messages` — thread history.
- `POST /api/chat/threads/{id}/messages` — send a message: persists the user
  message, runs the assistant loop (executing read tools, converting mutating
  tools to proposals), persists the assistant text, returns
  `{message, proposals[]}`. Returns **503** when `ANTHROPIC_API_KEY` is unset
  (same graceful degradation as Phase 4).

Applying a proposal adds **no new endpoint**: the frontend sends
`proposal.payload` to the existing `POST /api/products` or `POST /api/recipes`.

## Proposal shape
```
{ "type": "product" | "recipe", "payload": { ...exact body for the CRUD endpoint... } }
```
- `product` payload matches `productRequest` (name, category, brand, kcal100,
  protein100, fat100, carbs100, source).
- `recipe` payload matches `recipeRequest` (name, servings, instructions,
  meal_types, ingredients[{product_id, grams}]). The model is instructed to use
  product ids from a prior `list_products` read; ingredients whose product is
  unknown should be surfaced in the assistant's text so the user creates the
  product first.

## Frontend
The right panel placeholder becomes a working chat: select/create/delete thread,
message feed, input box. Proposals render as cards with "Применить" (calls the
matching CRUD hook; on success marks applied) and "Отклонить". Hooks in
`api/chat.ts`, types in `api/types.ts`. Reuses existing product/recipe create
hooks for apply.

## Testing
- `internal/assistant`: loop with a **mock Completer** — a read tool executes
  and is fed back; a mutating tool yields a proposal and does **not** mutate the
  DB; iteration cap respected. Tool-use parsing unit-tested as pure functions.
- `httpapi`: integration — create thread; send message with mock LLM →
  proposal in response and product **not** created; then apply via
  `POST /api/products` creates it; 503 without API key.
- No real Anthropic network calls in CI.

## Invariants preserved (per AGENTS.md)
LLM behind an interface (mockable); tool-use parsing pure; everything scoped by
`user_id` (threads → messages); mutations only via user confirmation; БЖУ keeps
its `complete` flag.

## Out of scope
Mutating tools beyond create (update/delete via chat); building meal plans or
store matching via chat (deferred — could be added as more tools later);
streaming responses; Kubernetes (Phase 6).
