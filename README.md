# WorldSignal

> A programmable global intelligence stream that converts the world's public
> information into clean, deduplicated, enriched, real-time **Signals**.

WorldSignal is **not** a news scraper. The durable asset is not the article — it
is the deduplicated, enriched, source-backed **Signal**. One Signal may have many
source articles behind it.

```
Sources → Ingestion → Normalization → Dedupe/Cluster → Enrichment → Distribution
```

## Stack (as built)

| Layer        | Choice                                             |
| ------------ | -------------------------------------------------- |
| Backend      | **Go** (net/http)                                  |
| API          | GraphQL (custom executor) + REST                   |
| DB access    | **pgx** over **Postgres only**                     |
| Jobs / queue | Postgres-backed queue (`internal/jobs`, no Redis)  |
| LLM          | **OpenAI** (graceful heuristic fallback w/o a key) |
| Frontend     | React + Vite                                       |

Everything that would normally need Redis/Kafka/a vector DB runs on Postgres.

> **History:** the backend was originally built in TypeScript (Fastify, graphql-yoga,
> Prisma, pg-boss) and migrated to Go with behavioural parity verified by a
> differential test harness (read ops byte-parity, mutations row-parity, pipeline
> shadow-run identical with the LLM disabled). See [MIGRATION_PLAN.md](MIGRATION_PLAN.md).

## Repo layout

```
WorldSignal/
  backend/      Go API + ingestion/enrichment/delivery workers
    cmd/server/         entrypoint (ROLE = all | api | worker)
    internal/           config, db, gql, httpapi, jobs, llm, pipeline, taxonomy, …
    schema/schema.prisma  canonical Postgres schema (applied via `prisma db push`)
  frontend/     React admin console (sources, signal explorer, taxonomy)
  docker-compose.yml    Postgres (+ Go backend) for local dev
```

## Quick start

```bash
# 1. Start Postgres
docker compose up -d postgres

# 2. Create the schema (uses the preserved Prisma schema; requires the prisma CLI)
#    or apply backend/schema/schema.prisma with your migration tool of choice.
DATABASE_URL=postgresql://worldsignal:worldsignal@localhost:5432/worldsignal \
  npx prisma db push --schema backend/schema/schema.prisma --skip-generate

# 3. Backend (Go)
cd backend
export DATABASE_URL=postgresql://worldsignal:worldsignal@localhost:5432/worldsignal
export OPENAI_API_KEY=          # optional; empty → heuristic enrichment
go run ./cmd/server             # API + queue workers + scheduler (ROLE=all)

# 4. Frontend (separate terminal)
cd frontend
npm install
npm run dev                     # http://localhost:5173 (proxies /graphql → :4000)
```

Or run the whole stack in Docker: `docker compose up --build`.

- REST API:    http://localhost:4000/v1/signals
- GraphQL:     http://localhost:4000/graphql
- Health:      http://localhost:4000/health

Without an `OPENAI_API_KEY`, enrichment falls back to a deterministic heuristic
summarizer + keyword taxonomy classifier so the pipeline still produces Signals.

## Configuration (env)

| Var | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | — (required) | Postgres connection string |
| `PORT` / `HOST` | `4000` / `0.0.0.0` | HTTP bind |
| `ROLE` | `all` | `all`, `api`, or `worker` (split API/workers) |
| `OPENAI_API_KEY` | empty | enables LLM enrichment when set |
| `OPENAI_MODEL` | `gpt-4o-mini` | model id |
| `WEBHOOK_SIGNING_SECRET` | `change-me-in-prod` | HMAC-SHA256 webhook signing |
| `SCHEDULER_TICK_MS` | `30000` | scheduler poll interval |

## What's implemented

- Source Registry (RSS/Atom) with priority + crawl frequency
- Postgres-backed queue + scheduler driving fetch / normalize / dedupe / enrich / deliver
- Raw evidence store → normalized Article → canonical **Signal**
- Exact dedupe (canonical URL, content hash) + token-similarity clustering
- Closed **WorldSignal Taxonomy** (LLM constrained to it, or keyword fallback)
- Webhook delivery (HMAC-signed, retry/backoff + dead-letter) + polling API
- GraphQL + REST query APIs, React admin console

## Testing

```bash
# Backend (Go) — DB-backed tests must be serialized; needs a Postgres test DB.
cd backend
go test ./... -p 1                       # all tests
go test ./... -p 1 -coverpkg=$(go list ./... | grep -vE '/cmd/server|/internal/dbtest|/internal/parity' | paste -sd, -) -coverprofile=cov.out   # coverage (≥95%)

# Frontend
cd frontend
npm test           # Vitest unit/component suite
npm run typecheck  # tsc --noEmit
npm run test:e2e   # Playwright against the Go backend (worldsignal_e2e DB)
```
