# Architecture

WorldSignal turns raw news/RSS feeds into canonical **Signals** (deduplicated,
classified events) and delivers them to subscribers. It is a Go backend + React
admin console over PostgreSQL.

## Components

```
                ┌──────────────┐     GraphQL /graphql      ┌───────────────────┐
   Browser ───► │  React SPA   │ ───────────────────────► │   Go HTTP server  │
   (admin)      │  (Mantine)   │     REST   /v1/*          │  internal/httpapi │
                └──────────────┘                            └─────────┬─────────┘
                                                                       │
                          ┌────────────────────────────────────────────┼───────────────┐
                          │                         │                    │               │
                    ┌─────▼─────┐            ┌───────▼──────┐     ┌───────▼──────┐ ┌──────▼──────┐
                    │ Postgres  │            │  Job queue   │     │  Scheduler   │ │ LLM gateway │
                    │  (pgx)    │◄───────────│  (ws_jobs)   │◄────│  (cron tick) │ │  (OpenAI)   │
                    └───────────┘            └──────┬───────┘     └──────────────┘ └─────────────┘
                                                    │
                                            ┌───────▼────────┐
                                            │  Workers       │  fetch → parse → enrich → cluster → deliver
                                            │ internal/jobs  │
                                            └────────────────┘
```

## Runtime roles

A single binary (`backend/cmd/server`) runs in one of three roles via `ROLE`:

- **api** — serves HTTP (GraphQL + REST). No background work.
- **worker** — runs the job workers + scheduler only.
- **all** (default) — both, for single-node/dev.

This lets the API scale independently of ingestion workers.

## Backend packages (`backend/internal`)

| Package | Responsibility |
|---|---|
| `httpapi` | HTTP server, GraphQL executor wiring, REST routes, resolvers, authz, audit |
| `gql` | Schemaless GraphQL executor (parse query → resolve → project selection) |
| `db` | pgx pool, all SQL, models, migrations (`MigrateAuth`, `MigrateContent`) |
| `auth` | bcrypt, opaque session tokens, role→permission matrix, identity context |
| `crypto` | AES-256-GCM encryption for secrets at rest (LLM keys) |
| `jobs` | Postgres-backed queue (`ws_jobs`), workers, scheduler |
| `pipeline` | fetch → parse → enrich → cluster → deliver stages |
| `ingestion` | RSS/Atom fetching + parsing (gofeed) |
| `llm` | Provider gateway (OpenAI) + heuristic fallback enricher |
| `sources` | Global source catalog: discovery, validation, seeding (`cmd/sourcetool`) |
| `taxonomy` | Closed classification vocabulary |

## Data flow (ingestion → delivery)

1. **Scheduler** enqueues `source.fetch` jobs for due sources (by `crawlFrequency`).
2. **Fetch** pulls the feed (RSS/Atom), writes immutable `RawItem` rows.
3. **Parse** normalizes raw items into `Article` rows (dedup by content hash).
4. **Enrich** classifies each article into title/summary/severity/tags using the
   LLM gateway, falling back to a deterministic heuristic when no key is active.
5. **Cluster** attaches articles to a new or existing `Signal`.
6. **Deliver** matches signals to `Subscription`s and enqueues `delivery.send`
   jobs (webhook/polling), recorded as `DeliveryEvent`.

## Frontend (`frontend/src`)

React + Vite + Mantine, React Router. `lib/api.ts` is the typed GraphQL client;
`lib/auth.tsx` holds the auth context; pages live under `pages/`, shared UI under
`components/`. Routes are gated by RBAC permissions on three layers: `RequireAuth`
(authentication), a per-route `RequirePerm` guard in `App.tsx` (direct navigation
to a route the user lacks permission for renders an "Access denied" page), and
`Layout.tsx` nav visibility. The server-side `authz` check on every resolver
remains the source of truth; the client guards are defence-in-depth/UX.

## Security model

- **AuthN**: opaque bearer session tokens (`Session` table), bcrypt password hashes.
- **AuthZ**: role→permission matrix (ADMIN/EDITOR/VIEWER); every resolver calls
  `authz(ctx, perm)`.
- **Secrets**: admin-managed LLM keys encrypted at rest (AES-GCM); the system key
  comes from the environment and is never returned by the API.
- **Audit**: security-relevant mutations recorded in `AuditLog`.
- **Injection**: all SQL is parameterized (pgx); feed-derived URLs are rendered via
  a safe-href guard in the UI. The API is token-based (no cookies) → CSRF N/A.

See [DATABASE.md](DATABASE.md), [API.md](API.md), [CONFIGURATION.md](CONFIGURATION.md),
[DEPLOYMENT.md](DEPLOYMENT.md), [RUNBOOK.md](RUNBOOK.md).
