# Frontend (TypeScript) — design

Client for backend Phases 1–3. Stack: React + Vite + TypeScript + Tailwind +
TanStack Query + React Router. Lives in `/frontend`.

## Layout (clean light dashboard)
Top bar (logo · email · logout); left sidebar nav; center content; a fixed
right **chat panel placeholder** ("чат — Фаза 5") so the layout is already
correct when Phase 5 lands.

## Screens
- **Auth** — login/register; JWT in `localStorage`; protected routes.
- **Dashboard** — profile (sex/height/age/activity/goal + meal_slots), weight
  entry + history, targets cards (БЖУ/ккал) and per-meal split from `/targets`.
- **Products** — table + create/edit form (per-100g БЖУ nullable; disabled
  "fill via LLM" placeholder for Phase 4).
- **Recipes** — list + form (ingredients = product+grams, meal_types, servings);
  shows total and per-serving macros.
- **Meal plan** — pick/create plan (dates), day × meal-slot grid, add recipe to
  a cell.
- **Shopping list** — per plan: aggregated grams + total/per-slot БЖУ + an
  "incomplete data" flag.

## Code structure
`src/api/` (typed client + per-resource TanStack Query hooks), `src/components/`
(Card, Button, Field, Table), `src/features/<screen>/`, `src/lib/auth.ts`.
Vite dev proxy `/api → localhost:8080`.

## Verification
`npm run build` (tsc + vite) clean; smoke against the running backend:
register → profile+weight → targets → product → recipe → plan → shopping list.

## Out of scope (later phases)
Store+LLM matching, recipe generation, LLM БЖУ pre-fill (Phase 4); functional
chat assistant (Phase 5); Kubernetes (Phase 6).
