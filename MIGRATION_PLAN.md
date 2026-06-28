# WorldSignal Backend Migration: TypeScript → Go

Migrate the WorldSignal backend from the TypeScript/Node stack (Fastify, graphql-yoga,
Prisma, pg-boss, OpenAI SDK) to Go, with **behavioural parity verified by differential
tests**, then delete the legacy TypeScript backend.

The Go backend reuses the **existing Postgres schema** (created by Prisma `db push`) so
row-level comparisons are meaningful and no schema is defined twice. Go reads/writes the
same tables via `pgx`/`sqlc`. CUID-style IDs are generated in Go to match Prisma's id shape.

## Parity definitions (the gates)

- **Read parity = byte-parity.** The Go and TS servers run against the *same* Postgres
  database with identical data; the same HTTP request must return byte-identical response
  bodies (status, JSON field order, date format, number format). Verified by a Go
  differential harness in `parity/` that hits both servers.
- **Mutation / REST-write parity = row-parity.** The same request issued against each
  backend (each on its own clean copy of the DB) must produce equivalent rows — equal on
  every column except system-generated, non-deterministic values (`id`, `createdAt`,
  `updatedAt`, timestamps), which are normalised before comparison.
- **Pipeline parity = shadow-run identical outputs with the LLM disabled.** With
  `OPENAI_API_KEY` empty (heuristic path), feeding the same fixtures through the TS and Go
  pipeline stages must yield identical persisted rows (normalised as above).

## Per-item gates

Every `[ ]` item below is "done" only when **all** of these pass for it:

1. `go build ./...` succeeds (backend).
2. `gofmt -l` reports nothing and `go vet ./...` is clean (lint).
3. The item's own parity test(s) pass.
4. It does not regress any previously-passing item.

**3-strike rule:** if a single item fails its gates 3 times, mark it `[!]`, stop, and
surface for human review.

## Global exit criteria (the goal)

- [ ] This plan exists and every item in Phases 0–4 is `[x]`.
- [ ] Go backend builds and lints clean.
- [ ] All parity tests pass (reads byte-parity; mutations/REST row-parity; pipeline shadow-run identical, LLM disabled).
- [ ] Go coverage ≥ 95% excluding generated code.
- [ ] Frontend test suite ≥ 95% coverage; frontend typecheck clean.
- [ ] Postgres-backed queue and dead-letter tests pass.
- [ ] End-to-end browser tests green against the Go backend.
- [ ] pg-boss removed; legacy TypeScript backend directory deleted.

---

## Phase 0 — Foundations & differential harness

- [x] 0.1 Go module + layout (`backend-go/`): `go.mod` (Go 1.26), `cmd/server`, `internal/{config,db,httpapi,gql,pipeline,jobs,llm,taxonomy,textutil,urlutil,logging,cuid,jsonx,ingestion}`.
- [x] 0.2 Config loader (`internal/config`) — parity with `src/config/env.ts` (DATABASE_URL required; PORT=4000, HOST=0.0.0.0, OPENAI_MODEL=gpt-4o-mini, ROLE∈{all,api,worker}, WEBHOOK_SIGNING_SECRET default; `HasOpenAI`). 100% cov.
- [x] 0.3 `textutil` port (stripHtml, normalizeText, tokenSet, tokenSetString, jaccard, contentHash, firstSentences) — tests mirror `lib/text.test.ts`. 100% cov.
- [x] 0.4 `urlutil` port (canonicalizeUrl: tracking-param strip, www/host-lowercase, sorted params, trailing-slash) — mirrors `lib/url.test.ts`. 100% cov.
- [x] 0.5 `taxonomy` port + `jsonx` (no-HTML-escape marshal). Taxonomy JSON verified **byte-identical** to TS `JSON.stringify(TAXONOMY)` and through the live TS `/v1/taxonomy`.
- [x] 0.6 `logging` scoped logger (info/warn/error/debug). 100% cov.
- [x] 0.7 CUID generator producing Prisma-shaped ids (`c` + base36); format + uniqueness tests.
- [x] 0.8 DB layer (`pgx` + pool) wired against the Prisma-created schema; `Source` model + `ListSources`/`GetSource` with byte-faithful types (`PrismaTime`, `RawJSON`).
- [x] 0.9 Test DB harness (`internal/dbtest`): connect to test DB, truncate app tables, seed taxonomy — mirrors `test-utils/db.ts`. Integration tests green.
- [x] 0.10 Differential harness (`internal/parity`): boot TS + Go servers (api role, LLM off) on a shared DB, GET/POST/PATCH helpers, byte-diff util. Smoke test boots the real TS server and confirms `/v1/taxonomy` byte-parity.

## Phase 1 — Read API parity (byte-parity)

- [x] 1.1 REST `GET /health`.
- [x] 1.2 REST `GET /v1/stats` (+ pending count) and GraphQL `stats`.
- [x] 1.3 REST `GET /v1/taxonomy` and GraphQL `taxonomy`.
- [x] 1.4 REST `GET /v1/sources` (`{data: rows}`, ordered priority asc, name asc — full Prisma row shape).
- [x] 1.5 GraphQL `sources` (projected fields, selection-order serialization).
- [x] 1.6 REST `GET /v1/signals` (filters: country, status, minConfidence, since, search, tags, limit/offset; `{data:[...]}` serializeSignal incl. tag label + relation).
- [x] 1.7 GraphQL `signals(filter,limit,offset)` (serializeSignal: tags{code,confidence}, sources{publisher,url,publishedAt}).
- [x] 1.8 REST `GET /v1/signals/:id` (+ 404 body).
- [x] 1.9 GraphQL `signal(id)` (+ null).
- [x] 1.10 REST `GET /v1/subscriptions` and GraphQL `subscriptions`.
- [x] 1.11 REST `GET /v1/deliveries`.
- [x] 1.12 CORS headers + OPTIONS 204 parity; GraphQL error-envelope parity (structural).

> **Findings (Phase 1):**
> - All JSON output goes through `jsonx` (no HTML escaping) to match `JSON.stringify`.
> - Timestamps serialize via `PrismaTime` as `…THH:MM:SS.000Z` (UTC, ms) to match Prisma/JS `Date`.
> - A custom selection-order GraphQL executor (`internal/gql`) replicates yoga's per-object field ordering. *Within* an object, field/alias order is preserved; *multiple top-level* fields are ordered by promise-completion in graphql-js (non-deterministic) — but every real query has a single top-level field, so this is moot.
> - The legacy TS `/graphql` **POST** hangs (Fastify drains the body before yoga reads it); GraphQL read-parity is verified over **GET** (identical response body). The Go backend handles POST correctly — a fix, exercised by Phase 4 e2e.

## Phase 2 — Mutations & REST writes (row-parity)

- [ ] 2.1 GraphQL `createSource` + REST `POST /v1/sources` (defaults; 201; 400 missing name/url; 409 dup url; enqueues immediate fetch).
- [ ] 2.2 GraphQL `setSourceEnabled` + REST `PATCH /v1/sources/:id` (enabled/priority/crawlFrequency).
- [ ] 2.3 GraphQL `triggerFetch` + REST `POST /v1/sources/:id/fetch` (enqueue, `{queued:true}` / `true`).
- [ ] 2.4 GraphQL `createSubscription` + REST `POST /v1/subscriptions` (default subscriber upsert; defaults; 201; 400 missing name).

## Phase 3 — Pipeline stages (shadow-run, LLM disabled)

- [ ] 3.1 `ingestion/rss` parity (DiscoveredItem extraction, field fallbacks, HTML strip) against fixture feeds — no network.
- [ ] 3.2 `fetchSource` (RawItem creation, (sourceId,sourceGuid) dedupe, source success/failure bookkeeping).
- [ ] 3.3 `normalize` (canonical URL, contentHash, tokenSet, exact dedupe by hash/url, RawItem status transitions, summary slice).
- [ ] 3.4 `cluster` (72h window, top-300, Jaccard ≥ 0.5 join as SUPPORTING + increment/lastSeenAt, else new Signal PRIMARY; idempotent on relink).
- [ ] 3.5 `llm/enrich` heuristic path (keyword scoring, severity regex, firstSentences summary, taxonomy-constrained tags, FALLBACK_CODE).
- [ ] 3.6 `enrichSignal` (representative pick, blended confidence 0.4/0.3/0.3, status by distinct sources, tag upsert, metadata) — LLM disabled.
- [ ] 3.7 `deliver.matchSubscriptions` (filter match: minConfidence, minSeverity rank, countries, tag prefix; envelope; unique (sub,signal)).
- [ ] 3.8 `deliver.sendDelivery` (POLLING immediate SENT; WEBHOOK POST + HMAC headers; success→SENT; fail→RETRYING/DEAD_LETTERED; signature parity) against a stub webhook server.
- [ ] 3.9 Full-pipeline shadow run: fixture → fetch→normalize→cluster→enrich→match→send, TS vs Go persisted-row diff identical.

## Phase 4 — Queue, workers, frontend, e2e, cleanup

- [ ] 4.1 Postgres-backed job queue in Go (replacement for pg-boss): queues, send with singletonKey, work/poll, retryLimit + backoff. Queue tests pass.
- [ ] 4.2 Dead-letter behaviour: delivery retries to limit then DEAD_LETTERED; dead-letter test passes.
- [ ] 4.3 Workers wiring (fetch→process→enrich→match→send fan-out) + scheduler (tick, crawlFrequency due check, singletonKey dedupe).
- [ ] 4.4 `cmd/server`: ROLE-based boot (api/worker/all), GraphQL + REST mounted, graceful shutdown.
- [ ] 4.5 Go coverage ≥ 95% (excluding generated code) — `go test ./... -coverprofile`, generated files filtered.
- [ ] 4.6 Frontend test harness (Vitest + RTL + coverage); tests for api.ts, badges, all views, App; **≥ 95% coverage**; `tsc` typecheck clean.
- [ ] 4.7 End-to-end browser tests (Playwright) green against the **Go** backend + frontend, seeded DB.
- [ ] 4.8 Point docker-compose / run scripts at the Go backend; update README.
- [ ] 4.9 Remove pg-boss (dependency + all references) from the repo.
- [ ] 4.10 Delete the legacy TypeScript backend directory (`backend/`); rename `backend-go/` → `backend/`; final full-suite green.
