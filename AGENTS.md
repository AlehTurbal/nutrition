# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A multi-user nutrition app: it computes daily БЖУ (protein/fat/carbs) + calorie
targets from a body profile and weight, lets users keep an editable products and
recipes database, build a weekly meal plan as a day × meal-slot grid, and derive
an aggregated shopping list. Backend in Go, frontend in TypeScript, PostgreSQL,
destined for Kubernetes. Russian-language UI; БЖУ is the local term for macros.

Built in phases (see `.claude/plans/` and `docs/superpowers/specs/`):
1. foundation (auth/JWT, profile, weight, target calc) — done
2. products + recipes with meal-type tags — done
3. meal slots, weekly plan grid, shopping need — done
4. store + LLM matching, recipe generation, БЖУ pre-fill — done
5. chat assistant (right panel, Claude tool-use, propose→confirm) — planned
6. Kubernetes manifests — planned

The frontend already reserves a right-side chat panel placeholder and a disabled
"fill via LLM" button for these future phases.

## Toolchain note (important)

There is **no local Go and no local psql** on this machine. Run all Go commands
through the `golang:1.25-alpine` Docker image, mounting `backend/` at `/app`.
Node (v25) **is** available locally for the frontend. Docker Desktop may need to
be started (`open -a Docker`) before container commands work.

## Commands

Backend (run from repo root):

```bash
# build + vet + unit tests
docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine \
  sh -c 'go build ./... && go vet ./... && go test ./...'

# single test
docker run --rm -v "$PWD/backend":/app -w /app golang:1.25-alpine \
  go test ./internal/mealplans/ -run TestBuildShoppingList -v
```

Integration tests live in `internal/httpapi` and are **skipped unless**
`NUTRITION_TEST_DATABASE_URL` is set. They drop and re-migrate all tables each
run. Start Postgres first, then point the test container at it via
`host.docker.internal`:

```bash
docker compose -f deploy/docker-compose.yml up -d postgres   # wait for healthy
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e NUTRITION_TEST_DATABASE_URL='postgres://nutrition:nutrition@host.docker.internal:5432/nutrition?sslmode=disable' \
  -v "$PWD/backend":/app -w /app golang:1.25-alpine \
  go test ./internal/httpapi/ -v
```

Full stack (backend is built in its own container — this is the only way to run
the server, since there is no local Go):

```bash
docker compose -f deploy/docker-compose.yml up --build   # backend on :8080
```

Frontend (from `frontend/`): `npm install`, `npm run dev` (Vite on :5173, proxies
`/api` → `:8080`), `npm run build` (runs `tsc -b` then `vite build`).

## Backend architecture

Layered, package-per-domain under `backend/internal/`. The dependency direction
is HTTP → store → pgx; calculation logic is pure and dependency-free.

- `nutrition/` — **pure** target math (no DB). `calc.go` (Mifflin–St Jeor BMR →
  TDEE via activity factor → goal adjustment → macro split) and `split.go`
  (`SplitEven` divides a daily target across meal slots). Unit-tested in
  isolation; this is where formula changes go.
- `users/` — accounts, profile (`meal_slots TEXT[]` = ordered meals/day),
  weight history. `Store` is the pgx-backed repo; `ErrNotFound`/`ErrEmailTaken`.
- `products/` — editable per-100g БЖУ. The per-100g fields are `*float64`
  (nullable) — a product's macros may be unknown, which propagates a "not
  complete" signal through macro computation.
- `recipes/` — `macros.go` has the **pure** `ComputeMacros` (a nil per-100g
  value counts as 0 but flips `Complete=false`); `store.go` does transactional
  create/update and verifies every ingredient product belongs to the user
  (`assertProductsOwned`).
- `mealplans/` — `store.go` (plans + items, day × meal-slot grid) and
  `shopping.go` (`ShoppingList` aggregates ingredient grams scaled by
  `item.servings / recipe.servings`, rolled into per-product totals, an overall
  total, and a per-slot breakdown — `buildShoppingList` is pure and unit-tested).
- `auth/` — bcrypt passwords, HS256 JWT (`Manager.Generate/Parse`), Bearer
  middleware that puts the user id in request context (`UserIDFromContext`),
  and a `Service` for register/login.
- `httpapi/` — chi router + handlers. `api.go` wires routes and holds the
  `Handlers` struct (one `*Store` per domain); `catalog.go` (products/recipes)
  and `mealplans.go` are the handler groups; `llmapi.go` holds the LLM
  handlers (`POST /api/stores/match`, `GET /api/stores/matches/{planID}`,
  `POST /api/recipes/generate`), returning 503 when no API key is set. All
  domain routes sit behind the JWT middleware.
- `llm/` — Anthropic client (`New(apiKey, model)`, `ErrNoAPIKey`) backing the
  store-matching + recipe-generation handlers; `stores/` persists matches.
- `db/` — `Connect` (pgxpool) and a **custom embedded migration runner**
  (`migrate.go` + `//go:embed migrations/*.sql`), applied in lexical order, each
  in its own transaction, tracked in `schema_migrations`. This is **not**
  golang-migrate. Add a new `NNNN_name.sql` to evolve the schema; migrations run
  automatically on server startup and at the start of each integration test.
- `config/` — env config; `DATABASE_URL` and `JWT_SECRET` required.
  `ANTHROPIC_API_KEY` is optional (LLM endpoints return 503 when unset);
  `LLM_MODEL` defaults to `claude-opus-4-8`. Compose passes both through.

### Conventions that matter

- **Every domain table carries `user_id`** and every query is scoped by it —
  this is how multi-user isolation is enforced. Preserve it on new tables/queries.
- **Cross-user references are validated** before writes (recipe ingredients,
  meal-plan items) and return a `400`-class domain error, not a DB error.
- **Keep calculation pure.** `nutrition.*` and `recipes.ComputeMacros` /
  `mealplans.buildShoppingList` take plain values and are tested without a DB.
  When adding logic, put the math in a pure function and call it from the store.
- БЖУ totals expose a `complete` boolean; surface it rather than hiding missing
  data.

## Frontend architecture

`frontend/src/`: `api/` is a typed layer — `client.ts` (fetch wrapper that
attaches the JWT and auto-clears it on 401) plus one file of TanStack Query
hooks per backend resource, with wire types in `api/types.ts` mirroring the Go
JSON. `lib/auth.ts` stores the token and notifies via a tiny pub/sub consumed by
`App.tsx` through `useSyncExternalStore` (no token ⇒ render `AuthPage`, else the
`Layout` + routed screens). `features/<screen>/` holds the screens;
`components/ui.tsx` is the shared primitive kit (Card/Button/Field/etc.).

When adding a backend endpoint, add its wire type to `api/types.ts` and a hook in
the matching `api/*.ts`; keep the screen components free of raw `fetch`.
