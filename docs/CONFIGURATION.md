# Configuration & Environment Variables

All backend configuration is via environment variables, validated at startup in
`backend/internal/config/config.go`. The process fails fast if `DATABASE_URL` is
missing or `ROLE`/`PORT`/`SCHEDULER_TICK_MS` are invalid.

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | **yes** | — | Postgres connection string (e.g. `postgresql://user@host:5432/worldsignal?sslmode=disable`). |
| `ROLE` | no | `all` | Process role: `all` \| `api` \| `worker`. |
| `HOST` | no | `0.0.0.0` | Bind address for the HTTP server. |
| `PORT` | no | `4000` | HTTP port. |
| `OPENAI_API_KEY` | no | _(empty)_ | System LLM key. Empty ⇒ heuristic enrichment. Overridden at runtime by an active admin-managed key. |
| `OPENAI_MODEL` | no | `gpt-4o-mini` | Default chat model for enrichment. |
| `WEBHOOK_SIGNING_SECRET` | no | `change-me-in-prod` | Signs delivery webhooks **and** derives the AES key that encrypts admin LLM keys at rest. **Set a strong value in production** — changing it invalidates stored LLM keys. |
| `SCHEDULER_TICK_MS` | no | `30000` | Scheduler poll interval (ms). |
| `ADMIN_EMAIL` | no | `admin@worldsignal.local` | Seeded on first boot when no users exist. |
| `ADMIN_PASSWORD` | no | `admin12345` | Seeded admin password — **change immediately in production**. |

## Local secrets

For local development, put secrets in `backend/.env.local` (git-ignored). `dev.sh`
sources it automatically:

```bash
# backend/.env.local
OPENAI_API_KEY="sk-proj-…"
OPENAI_MODEL="gpt-4o-mini"
WEBHOOK_SIGNING_SECRET="dev-secret"
```

Never commit real secrets. `.gitignore` already excludes `.env` and `.env.local`.

## Frontend

The SPA has no runtime secrets. The dev server proxies `/graphql` and `/v1` to the
backend (`WS_BACKEND`, default `http://localhost:4000`) — see `frontend/vite.config.ts`.

## Databases used in this repo

| Name | Purpose |
|---|---|
| `worldsignal` | local development |
| `worldsignal_test` | Go unit/integration tests (`TEST_DATABASE_URL`) |
| `worldsignal_e2e` | Playwright end-to-end runs |
