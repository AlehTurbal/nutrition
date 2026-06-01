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
