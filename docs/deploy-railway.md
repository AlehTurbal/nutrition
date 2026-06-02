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

```bash
railway add --service backend --json
railway add --service frontend --json
```

> The service **must be named `backend`** — the frontend references it by name
> in `BACKEND_URL` below. If you name it differently, update that variable.

### Scope each service to its subdirectory + Dockerfile builder

`railway up` uploads the **whole git repo root**, so Railway needs to know which
subdirectory each service builds from. Without this it falls back to Railpack on
the repo root and the build fails (`Railpack could not determine how to build`).
Set the root directory **and** force the Dockerfile builder. Resolve the service
IDs first (`railway service list --json`), then patch config (names don't work in
JSON patches — use IDs):

```bash
railway environment edit --json <<'JSON'
{"services":{
  "<BACKEND_SERVICE_ID>":{"source":{"rootDirectory":"/backend"},"build":{"builder":"DOCKERFILE","dockerfilePath":"Dockerfile"}},
  "<FRONTEND_SERVICE_ID>":{"source":{"rootDirectory":"/frontend"},"build":{"builder":"DOCKERFILE","dockerfilePath":"Dockerfile"}}
}}
JSON
```

(Equivalently in the dashboard: each service → Settings → **Root Directory** =
`backend` / `frontend`, **Builder** = Dockerfile. `dockerfilePath` is relative to
the root directory, so it's just `Dockerfile`.)

## 4. Configure environment variables

`PORT` is **not** auto-injected by Railway — set it explicitly so the apps and the
`BACKEND_URL` reference are deterministic.

**Backend service:**

```bash
railway variable set PORT=8080 --service backend --skip-deploys
railway variable set 'DATABASE_URL=${{Postgres.DATABASE_URL}}' --service backend --skip-deploys
SECRET=$(openssl rand -base64 48)
printf "%s" "$SECRET" | railway variable set JWT_SECRET --stdin --service backend --skip-deploys
# LLM_MODEL is optional; the backend defaults to claude-opus-4-8.
```

Set your Anthropic key (kept out of shell history via stdin):

```bash
printf "%s" "sk-ant-..." | railway variable set ANTHROPIC_API_KEY --stdin --service backend
```

Without `ANTHROPIC_API_KEY` the app runs fine but LLM/chat endpoints return 503.

**Frontend service:**

```bash
railway variable set PORT=8080 --service frontend --skip-deploys
railway variable set 'BACKEND_URL=http://${{backend.RAILWAY_PRIVATE_DOMAIN}}:${{backend.PORT}}' --service frontend --skip-deploys
```

nginx listens on `$PORT`; the `${{backend.PORT}}` reference resolves to the `PORT`
you set on the backend (8080).

## 5. Deploy

From the **repo root** (so the upload includes each service's subdirectory):

```bash
railway up --service backend  --detach -m "backend deploy"
railway up --service frontend --detach -m "frontend deploy"
```

Generate a public domain on the **frontend** only, routed to its port:

```bash
railway domain --service frontend --port 8080 --json
```

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

- **Build fails with `Railpack could not determine how to build the app`** and the
  analyzed tree shows the whole repo root: the service's root directory / Dockerfile
  builder isn't set. Apply the config patch in step 3.
- **Frontend 502 on everything (including `/`):** the nginx container is
  crashlooping — check `railway logs --service frontend`. A known cause is
  `invalid port in resolver "fd12::10"`: Railway's internal DNS is IPv6 and nginx
  needs it bracketed (`[fd12::10]`). The entrypoint (`frontend/docker-entrypoint.sh`)
  already brackets IPv6 resolvers; this note is here in case the logic regresses.
- **Frontend 502 only on `/api` (static `/` works):** the backend isn't reachable
  on the private network. Confirm it's named `backend`, deployed and healthy
  (`railway logs --service backend` should show `listening on :8080`), and that
  `BACKEND_URL` resolves to `…RAILWAY_PRIVATE_DOMAIN…:…PORT…`.
- **Backend boot loop logging `DATABASE_URL is required`:** the `DATABASE_URL`
  reference variable isn't set or the Postgres service isn't linked.
- **`401` after login works briefly:** `JWT_SECRET` changed between deploys; set a
  fixed secret, don't regenerate it.
