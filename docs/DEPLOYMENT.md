# Deployment

## Build artifacts

- **Backend**: a single static Go binary.
  ```bash
  cd backend && go build -o ws-server ./cmd/server
  ```
- **Frontend**: a static bundle in `frontend/dist/`.
  ```bash
  cd frontend && npm ci && npm run build
  ```
  Serve `dist/` from any static host/CDN; proxy `/graphql` and `/v1/*` to the backend.

A `docker-compose.yml` at the repo root brings up Postgres + the app for local use.

## Topology

Run the backend in split roles behind a load balancer:

- **API tier** (`ROLE=api`): N replicas, stateless, behind the LB. Scales with traffic.
- **Worker tier** (`ROLE=worker`): 1+ replicas for ingestion/enrichment/delivery.
- **Single node** (`ROLE=all`): fine for dev/small installs.

All tiers share one Postgres. Migrations run automatically on boot (idempotent), so
rolling deploys are safe; for many replicas, prefer running one instance first.

## Required configuration in production

- `DATABASE_URL` — managed Postgres with TLS.
- `WEBHOOK_SIGNING_SECRET` — strong, stable secret (rotating it invalidates stored
  LLM key ciphertext; re-enter keys after rotation).
- `ADMIN_EMAIL` / `ADMIN_PASSWORD` — set, then change the password after first login.
- `OPENAI_API_KEY` (optional) — system enrichment key; admins can add their own via
  **Settings**. With no key, enrichment uses the deterministic heuristic.

See [CONFIGURATION.md](CONFIGURATION.md) for the full variable list.

## Health & readiness

- Liveness/readiness: `GET /health` → `200 {"status":"ok"}`.
- The process exits non-zero on invalid config or an unreachable database at boot.

## Database

- Provision Postgres 14+. Grant the app role DDL (it runs idempotent migrations on boot).
- Back up regularly; the schema is migration-tooling-agnostic (`IF NOT EXISTS`).

## Rollout checklist

1. Apply DB (automatic on first boot of the new version).
2. Deploy worker tier, then API tier.
3. Verify `/health`, log in, confirm **Settings → LLM status** and a test fetch.
4. Roll back by redeploying the prior image (schema changes are additive/compatible).
