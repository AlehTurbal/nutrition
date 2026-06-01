# Railway Deployment — Design Spec

**Date:** 2026-06-01
**Status:** Approved, ready for implementation plan

## Goal

Deploy the nutrition app to Railway as **two services + a managed Postgres**, in
a single Railway project built from this monorepo. The LLM chat/recipe features
are in scope (an Anthropic API key will be configured).

## Constraints / context

- The SPA (`frontend/`) calls the API via **relative `/api` paths** with no
  configurable base URL. The frontend must therefore be served from the **same
  origin** as the API. We achieve this with an nginx reverse proxy on the
  frontend service rather than CORS — the frontend code stays untouched.
- The backend (`backend/`) is a Go chi API serving `/api/*` and `/health`,
  listening on `$PORT` (already handled by `config.Load()`, default 8080). It
  runs its own embedded DB migrations on startup.
- Required backend env: `DATABASE_URL`, `JWT_SECRET`. Optional:
  `ANTHROPIC_API_KEY` (LLM endpoints return 503 without it), `LLM_MODEL`
  (defaults to `claude-opus-4-8`), `JWT_TTL`.
- Existing `backend/Dockerfile` (multi-stage → distroless) is reused unchanged.
- Local Node is v25; `frontend/package-lock.json` exists.

## Topology

```
                         ┌─────────────────────────────┐
   browser ──HTTPS──▶    │  frontend service (nginx)    │  ← public domain
                         │  serves Vite dist/           │
                         │  proxies /api, /health  ─────┼──┐ (private net)
                         └─────────────────────────────┘  │
                         ┌─────────────────────────────┐  │
                         │  backend service (Go)        │ ◀┘ backend.railway.internal
                         │  /api/*, /health  on $PORT   │
                         └──────────────┬──────────────┘
                                        │ DATABASE_URL (private)
                         ┌──────────────▼──────────────┐
                         │  Postgres (Railway plugin)   │
                         └─────────────────────────────┘
```

## Components

### 1. Backend service
- **Build:** existing `backend/Dockerfile`, no code changes.
- **Railway Root Directory:** `backend/`.
- **Builder:** pinned to Dockerfile via `backend/railway.json`.
- **Listens on `$PORT`** (Railway-injected).
- **No public domain** — reachable only at `backend.railway.internal` on the
  private network.
- **Env vars:**
  - `DATABASE_URL` → reference to the Postgres plugin connection string.
  - `JWT_SECRET` → generated secret pasted into Railway.
  - `ANTHROPIC_API_KEY` → user's key.
  - `LLM_MODEL` → optional; omit to use default.
- Migrations apply automatically on startup (already wired in `main.go`).

### 2. Frontend service (new files)
- **`frontend/Dockerfile`** — two stages:
  - *build:* `node:25-alpine`, `npm ci && npm run build` → `dist/`.
  - *runtime:* `nginx:alpine`, copies `dist/` + templated nginx config +
    entrypoint.
- **`frontend/nginx.conf.template`**:
  - `location / { try_files $uri /index.html; }` for SPA client-side routing.
  - `location /api/` and `location /health` → `proxy_pass` to `$BACKEND_URL`.
  - `listen $PORT;` (Railway-injected for this service).
  - Uses a `resolver` + variable upstream so the internal backend hostname is
    resolved per-request (nginx otherwise caches DNS at startup and breaks when
    the backend restarts).
- **`frontend/docker-entrypoint.sh`** — runs `envsubst` to fill `$PORT` and
  `$BACKEND_URL` into the nginx config, then `exec nginx -g 'daemon off;'`.
- **`frontend/.dockerignore`** — ignores `node_modules`, `dist` (always build
  fresh in the image), local env files.
- **Railway Root Directory:** `frontend/`. **Owns the public domain.**
- **Builder:** pinned to Dockerfile via `frontend/railway.json`.
- **Env var:** `BACKEND_URL` from a Railway reference variable, e.g.
  `http://${{backend.RAILWAY_PRIVATE_DOMAIN}}:${{backend.PORT}}`.

### 3. Postgres
- Railway managed **PostgreSQL** plugin. Provides `DATABASE_URL` on the private
  network, referenced into the backend service.

## New / changed files

| File | Purpose |
|------|---------|
| `backend/railway.json` | Pin builder to Dockerfile. |
| `frontend/Dockerfile` | Build Vite bundle, serve via nginx. |
| `frontend/nginx.conf.template` | SPA fallback + `/api` & `/health` proxy. |
| `frontend/docker-entrypoint.sh` | `envsubst` templating + launch nginx. |
| `frontend/.dockerignore` | Exclude `node_modules`, `dist`, env files. |
| `frontend/railway.json` | Pin builder to Dockerfile. |
| `docs/deploy-railway.md` | Exact deploy steps, env var list, secret-gen. |

No backend Go code changes. No frontend application code changes.

## Deploy flow (operator runs)

1. `railway login`
2. `railway init` — create the project.
3. `railway add --database postgres` — add managed Postgres.
4. Create two services with root dirs `backend/` and `frontend/` (CLI or
   dashboard).
5. Set env vars / reference variables per component above.
6. `railway up` per service; generate a public domain on the **frontend**
   service only.

Exact commands live in `docs/deploy-railway.md`.

## Verification

- **Local build sanity:** `docker build` succeeds for both
  `backend/Dockerfile` and `frontend/Dockerfile`.
- **Post-deploy smoke test (against the public frontend domain):**
  - `GET /health` → 200 (proves proxy → backend path works).
  - Register + login round-trip succeeds (proves DB + JWT).
  - Chat request succeeds (proves `ANTHROPIC_API_KEY` wired).

## Out of scope

- CI/CD automation (GitHub auto-deploy hooks) beyond the manual flow.
- Single-service consolidation (explicitly chose two services).
- Frontend code changes for a configurable API base URL (not needed with the
  proxy).
- Kubernetes manifests (separate planned phase).
