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
| Backend      | Node.js + TypeScript + Fastify                     |
| API          | GraphQL (Yoga) + REST                              |
| ORM / DB     | Prisma + **Postgres only**                         |
| Jobs / queue | **pg-boss** (Postgres-backed, no Redis)            |
| LLM          | **OpenAI** (graceful heuristic fallback w/o a key) |
| Frontend     | React + Vite                                       |

Everything that would normally need Redis/Kafka/a vector DB runs on Postgres for
this MVP, exactly as requested.

## Repo layout

```
WorldSignal/
  backend/      Node API + ingestion/enrichment/delivery workers
  frontend/     React admin console (sources, signal explorer, taxonomy)
  docker-compose.yml   Postgres for local dev
```

## Quick start

```bash
# 1. Start Postgres
docker compose up -d

# 2. Backend
cd backend
cp .env.example .env          # edit OPENAI_API_KEY if you have one (optional)
npm install
npm run db:push               # create schema
npm run db:seed               # taxonomy + sample RSS sources
npm run dev                   # starts API + pg-boss workers + scheduler

# 3. Frontend (separate terminal)
cd frontend
npm install
npm run dev                   # http://localhost:5173
```

- REST API:    http://localhost:4000/v1/signals
- GraphQL:     http://localhost:4000/graphql
- Health:      http://localhost:4000/health

Without an `OPENAI_API_KEY`, enrichment falls back to a deterministic heuristic
summarizer + keyword taxonomy classifier so the pipeline still produces Signals.

## What's implemented (Phase 1)

- Source Registry (RSS/Atom) with priority + crawl frequency
- pg-boss scheduler + fetch / normalize / dedupe / enrich / deliver workers
- Raw evidence store → normalized Article → canonical **Signal**
- Exact dedupe (canonical URL, source GUID, content hash) + lightweight
  token-similarity clustering into Signals
- Closed **WorldSignal Taxonomy** (LLM constrained to it, or keyword fallback)
- Webhook delivery (HMAC-signed, retry/backoff via pg-boss) + polling API
- GraphQL + REST query APIs, React admin console

See `backend/src` module folders for the service breakdown.
