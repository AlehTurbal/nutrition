# Railway Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy the nutrition app to Railway as two services (Go backend + nginx-served SPA) plus managed Postgres, with the SPA's relative `/api` calls proxied to the backend over Railway's private network.

**Architecture:** Backend reuses its existing distroless `Dockerfile` unchanged and stays private (`backend.railway.internal`). A new frontend service builds the Vite bundle and serves it via nginx, reverse-proxying `/api` and `/health` to the backend — so the browser sees one origin and the frontend code needs no changes. Each service pins its builder to its Dockerfile via a `railway.json`; Postgres is a Railway plugin referenced into the backend.

**Tech Stack:** Go 1.25 (backend, unchanged), Node 25 + Vite (frontend build), nginx (frontend runtime), Railway (Dockerfile builder, private networking, managed Postgres), Docker (local build verification — no local Go/psql per AGENTS.md).

---

## File Structure

| File | Responsibility |
|------|----------------|
| `backend/railway.json` | Pin the backend service builder to its Dockerfile. |
| `frontend/.dockerignore` | Keep `node_modules`, `dist`, env files out of the build context (always build fresh). |
| `frontend/Dockerfile` | Two-stage: build Vite bundle (node), serve via nginx. |
| `frontend/nginx.conf.template` | SPA `try_files` fallback + `/api` & `/health` reverse proxy with runtime DNS resolution. |
| `frontend/docker-entrypoint.sh` | Derive the DNS resolver, `envsubst` the template, launch nginx. |
| `frontend/railway.json` | Pin the frontend service builder to its Dockerfile. |
| `docs/deploy-railway.md` | Operator runbook: exact CLI steps, env vars, secret generation, smoke test. |

No backend Go code changes. No frontend application code changes.

---

## Task 1: Pin the backend builder

**Files:**
- Create: `backend/railway.json`

- [ ] **Step 1: Create `backend/railway.json`**

```json
{
  "$schema": "https://railway.com/railway.schema.json",
  "build": {
    "builder": "DOCKERFILE",
    "dockerfilePath": "Dockerfile"
  },
  "deploy": {
    "restartPolicyType": "ON_FAILURE",
    "restartPolicyMaxRetries": 10
  }
}
```

- [ ] **Step 2: Validate it is well-formed JSON**

Run: `python3 -m json.tool backend/railway.json`
Expected: the file is printed back with no error.

- [ ] **Step 3: Confirm the referenced Dockerfile path is correct**

Run: `test -f backend/Dockerfile && echo OK`
Expected: `OK` (the path in `dockerfilePath` is relative to the service root dir `backend/`).

- [ ] **Step 4: Commit**

```bash
git add backend/railway.json
git commit -m "Railway: pin backend service to its Dockerfile builder"
```

---

## Task 2: Frontend build context ignore file

**Files:**
- Create: `frontend/.dockerignore`

- [ ] **Step 1: Create `frontend/.dockerignore`**

```
node_modules
dist
.env
.env.*
*.log
.DS_Store
tsconfig.tsbuildinfo
```

- [ ] **Step 2: Confirm the committed `dist/` will be excluded from the image**

Run: `grep -qx dist frontend/.dockerignore && echo OK`
Expected: `OK` (the image always rebuilds the bundle rather than copying a stale committed `dist/`).

- [ ] **Step 3: Commit**

```bash
git add frontend/.dockerignore
git commit -m "Railway: add frontend .dockerignore (always build fresh)"
```

---

## Task 3: Frontend nginx config template

**Files:**
- Create: `frontend/nginx.conf.template`

The template uses three substituted placeholders — `${PORT}`, `${BACKEND_URL}`, `${RESOLVER}` — filled at container start. All other `$...` tokens are nginx runtime variables and must survive untouched (the entrypoint restricts `envsubst` to exactly those three names).

`proxy_pass` uses a variable upstream (`$backend`) plus an explicit `resolver`, which forces nginx to resolve the backend's private hostname **per request** instead of caching one IP at startup — necessary because the backend's private IPv6 changes across redeploys.

- [ ] **Step 1: Create `frontend/nginx.conf.template`**

```nginx
server {
    listen ${PORT};
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    resolver ${RESOLVER} valid=10s ipv6=on;

    # SPA client-side routing: serve the file if it exists, else index.html.
    location / {
        try_files $uri $uri/ /index.html;
    }

    # API + health proxied to the backend over Railway's private network.
    # Variable upstream + resolver = runtime DNS resolution per request.
    location /api/ {
        set $backend ${BACKEND_URL};
        proxy_pass $backend$request_uri;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /healthz {
        set $backend ${BACKEND_URL};
        proxy_pass $backend$request_uri;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }
}
```

- [ ] **Step 2: Confirm only the three intended placeholders use `${...}` form**

Run: `grep -oE '\$\{[A-Z_]+\}' frontend/nginx.conf.template | sort -u`
Expected exactly these three lines:
```
${BACKEND_URL}
${PORT}
${RESOLVER}
```
(Any other `${...}` would be wrongly substituted; nginx variables like `$uri`, `$host`, `$request_uri` must use the bare `$name` form, which they do.)

- [ ] **Step 3: Commit**

```bash
git add frontend/nginx.conf.template
git commit -m "Railway: add nginx SPA + API-proxy config template"
```

---

## Task 4: Frontend container entrypoint

**Files:**
- Create: `frontend/docker-entrypoint.sh`

The entrypoint reads the container's DNS nameserver from `/etc/resolv.conf` (Railway's private DNS), substitutes the three placeholders into the live nginx config with an explicit allowlist (so nginx's own `$variables` are preserved), then execs nginx.

- [ ] **Step 1: Create `frontend/docker-entrypoint.sh`**

```sh
#!/bin/sh
set -eu

# Railway provides PORT and (via a reference variable) BACKEND_URL.
: "${PORT:?PORT is required}"
: "${BACKEND_URL:?BACKEND_URL is required}"

# Derive the DNS resolver nginx should use for runtime upstream resolution.
RESOLVER="$(awk '/^nameserver/ { print $2; exit }' /etc/resolv.conf)"
RESOLVER="${RESOLVER:-127.0.0.11}"
export PORT BACKEND_URL RESOLVER

# Substitute ONLY our three placeholders; leave nginx $variables intact.
envsubst '${PORT} ${BACKEND_URL} ${RESOLVER}' \
  < /etc/nginx/nginx.conf.template \
  > /etc/nginx/conf.d/default.conf

# Surface the rendered config in logs for debugging, then validate + run.
nginx -t
exec nginx -g 'daemon off;'
```

- [ ] **Step 2: Mark it executable**

Run: `chmod +x frontend/docker-entrypoint.sh && test -x frontend/docker-entrypoint.sh && echo OK`
Expected: `OK`

- [ ] **Step 3: Confirm the resolver fallback and required-var guards are present**

Run: `grep -c 'PORT is required\|BACKEND_URL is required\|127.0.0.11' frontend/docker-entrypoint.sh`
Expected: `3`

- [ ] **Step 4: Commit**

```bash
git add frontend/docker-entrypoint.sh
git commit -m "Railway: add nginx entrypoint (resolver + envsubst templating)"
```

---

## Task 5: Frontend Dockerfile

**Files:**
- Create: `frontend/Dockerfile`

- [ ] **Step 1: Create `frontend/Dockerfile`**

```dockerfile
# syntax=docker/dockerfile:1

# --- build the Vite bundle ---
FROM node:25-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# --- serve via nginx ---
FROM nginx:alpine
# gettext provides envsubst (already present in nginx:alpine, installed defensively).
RUN apk add --no-cache gettext
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/nginx.conf.template
COPY docker-entrypoint.sh /docker-entrypoint-railway.sh
RUN chmod +x /docker-entrypoint-railway.sh
# Remove the stock default server so only our rendered config is loaded.
RUN rm -f /etc/nginx/conf.d/default.conf
ENTRYPOINT ["/docker-entrypoint-railway.sh"]
```

- [ ] **Step 2: Build the image locally**

Run: `docker build -t nutrition-frontend ./frontend`
Expected: build completes through `npm run build` and the nginx stage; final line `naming to ... nutrition-frontend`.

- [ ] **Step 3: Verify the rendered nginx config is valid with dummy env**

Run:
```bash
docker run --rm -e PORT=8080 -e BACKEND_URL=http://backend.railway.internal:8080 \
  --entrypoint sh nutrition-frontend -c \
  'RESOLVER=$(awk "/^nameserver/{print \$2; exit}" /etc/resolv.conf); \
   export PORT BACKEND_URL RESOLVER; \
   envsubst "\${PORT} \${BACKEND_URL} \${RESOLVER}" < /etc/nginx/nginx.conf.template > /etc/nginx/conf.d/default.conf; \
   nginx -t'
```
Expected: `nginx: configuration file /etc/nginx/nginx.conf test is successful`.

- [ ] **Step 4: Commit**

```bash
git add frontend/Dockerfile
git commit -m "Railway: add frontend Dockerfile (Vite build + nginx serve)"
```

---

## Task 6: Pin the frontend builder

**Files:**
- Create: `frontend/railway.json`

- [ ] **Step 1: Create `frontend/railway.json`**

```json
{
  "$schema": "https://railway.com/railway.schema.json",
  "build": {
    "builder": "DOCKERFILE",
    "dockerfilePath": "Dockerfile"
  },
  "deploy": {
    "restartPolicyType": "ON_FAILURE",
    "restartPolicyMaxRetries": 10
  }
}
```

- [ ] **Step 2: Validate it is well-formed JSON**

Run: `python3 -m json.tool frontend/railway.json`
Expected: the file is printed back with no error.

- [ ] **Step 3: Commit**

```bash
git add frontend/railway.json
git commit -m "Railway: pin frontend service to its Dockerfile builder"
```

---

## Task 7: Verify the backend image still builds

This task changes no code — it confirms the existing backend Dockerfile builds cleanly in the same Docker-only workflow Railway will use, so a deploy failure can be ruled out locally first.

- [ ] **Step 1: Build the backend image**

Run: `docker build -t nutrition-backend ./backend`
Expected: build completes the `go build` and distroless stages; final line `naming to ... nutrition-backend`.

- [ ] **Step 2: Confirm it fails fast without config (proves the binary runs)**

Run: `docker run --rm nutrition-backend; echo "exit=$?"`
Expected: it logs `DATABASE_URL is required` and exits non-zero (`exit=1`). This proves the entrypoint executes; Railway will supply the env vars.

---

## Task 8: Operator runbook

**Files:**
- Create: `docs/deploy-railway.md`

- [ ] **Step 1: Create `docs/deploy-railway.md`**

````markdown
# Deploying to Railway

Two services (backend + frontend) and a managed Postgres in one Railway project,
all built from this monorepo. The frontend serves the SPA and reverse-proxies
`/api` and `/healthz` to the backend over Railway's private network, so only the
frontend gets a public domain.

## Prerequisites

- A Railway account and the CLI: `npm i -g @railway/cli` (or `brew install railway`).
- This repo pushed to GitHub (optional, only if you prefer dashboard deploys).

## 1. Log in and create the project

```bash
railway login
railway init        # name the project, e.g. "nutrition"
```

## 2. Add managed Postgres

```bash
railway add --database postgres
```

This creates a `Postgres` service exposing `DATABASE_URL` for reference.

## 3. Create the two app services

In the Railway dashboard (Project → New → Empty Service), create two services and
set each one's **Settings → Root Directory**:

| Service name | Root Directory | Public domain? |
|--------------|----------------|----------------|
| `backend`    | `backend`      | No             |
| `frontend`   | `frontend`     | Yes            |

Each service auto-detects its `railway.json` and builds the Dockerfile there.

> The service **must be named `backend`** — the frontend references it by name
> in `BACKEND_URL` below. If you name it differently, update that variable.

## 4. Configure environment variables

**Backend service** (Settings → Variables):

| Variable | Value |
|----------|-------|
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` (reference variable) |
| `JWT_SECRET` | a strong random secret — generate with `openssl rand -base64 48` |
| `ANTHROPIC_API_KEY` | your Anthropic API key |
| `LLM_MODEL` | optional; omit to use the default `claude-opus-4-8` |

Railway injects `PORT` automatically; the backend already reads it.

**Frontend service** (Settings → Variables):

| Variable | Value |
|----------|-------|
| `BACKEND_URL` | `http://${{backend.RAILWAY_PRIVATE_DOMAIN}}:${{backend.PORT}}` |

Railway injects `PORT` automatically; nginx listens on it.

## 5. Deploy

From each service directory (or use `railway service` to select), push the code:

```bash
railway up        # run once per service, or trigger deploys from the dashboard
```

Then, on the **frontend** service only: Settings → Networking → **Generate Domain**.

## 6. Smoke test

Replace `<frontend-domain>` with the generated domain:

```bash
# 1. Proxy → backend path works:
curl -fsS https://<frontend-domain>/healthz && echo

# 2. DB + JWT round-trip:
curl -fsS -X POST https://<frontend-domain>/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"smoke@example.com","password":"smoke-test-123"}' | head -c 200; echo
```

Then open `https://<frontend-domain>` in a browser, register/log in, and send a
chat message to confirm `ANTHROPIC_API_KEY` is wired (LLM endpoints return 503 if
the key is missing).

## Troubleshooting

- **Frontend 502 on `/api`:** the backend isn't reachable on the private network.
  Confirm the backend service is named `backend`, is deployed and healthy, and
  that `BACKEND_URL` resolves to `…RAILWAY_PRIVATE_DOMAIN…:…PORT…`. Check the
  rendered config in the frontend deploy logs (`nginx -t` runs at startup).
- **Backend boot loop logging `DATABASE_URL is required`:** the `DATABASE_URL`
  reference variable isn't set or the Postgres service isn't linked.
- **`401` after login works briefly:** `JWT_SECRET` changed between deploys; set a
  fixed secret, don't regenerate it.
````

- [ ] **Step 2: Confirm the runbook references match the real endpoints**

Run: `grep -n "auth/register\|healthz" backend/internal/httpapi/api.go`
Expected: `/healthz` (root-level) and `/auth/register` (nested under the `/api` route → `/api/auth/register`) both appear. Adjust the runbook paths if the printed routes differ.

- [ ] **Step 3: Commit**

```bash
git add docs/deploy-railway.md
git commit -m "Railway: add deployment runbook"
```

---

## Self-Review notes

- **Spec coverage:** backend service (Task 1, 7), frontend service files —
  `.dockerignore` (Task 2), nginx template (Task 3), entrypoint (Task 4),
  Dockerfile (Task 5), `railway.json` (Task 6); Postgres + env wiring + deploy
  flow + smoke test (Task 8 runbook). All spec "New/changed files" rows are
  covered.
- **DNS-caching gotcha** from the spec is implemented in Tasks 3–4 (variable
  upstream + `resolver` + per-request resolution).
- **No backend code changes** — confirmed; the existing Dockerfile is reused and
  only build-verified (Task 7).
- **Register route path:** Task 8 Step 2 verifies the exact path against
  `api.go` and tells the operator to adjust the runbook if the prefix differs —
  guarding against a stale example URL.
