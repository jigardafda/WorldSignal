# Streaming subscriptions & richer filters — design

Status: approved (Phase A in progress). Date: 2026-07-05.

## Goal

Make WorldSignal subscriptions consumable in real time and self-service:
- add **SSE** and **WebSocket** delivery channels alongside webhook/email/polling;
- a **richer, shared filter model** (more than tags/countries/severity);
- (Phase B) a full-screen, interactive subscription UI with a visual filter
  builder and live per-language code examples;
- a tested **`example-clients/`** directory (Python + TypeScript to start).

Decisions (locked): stored Subscriptions are the entity; programmatic clients
authenticate with **API keys** (`signals:read` scope, existing rate limits);
delivery is **phased** (Phase A backend, Phase B frontend); code examples cover
7 languages, runnable clients start with Python + TypeScript.

## Core idea: delivery rows are the durable log; channels are transports

`MatchSubscriptions` already writes a `DeliveryEvent` per matched signal. That
row is the single source of truth for every **pull-family** channel:

| Channel   | Family | Transport                                             |
|-----------|--------|-------------------------------------------------------|
| WEBHOOK   | push   | server HMAC-POSTs to the configured URL               |
| EMAIL     | push   | SMTP (instant / digest)                               |
| POLLING   | pull   | `GET /v1/deliveries?subscription=X&since=<cursor>`    |
| SSE       | pull   | `GET /v1/stream/sse?subscription=X` — replay + live   |
| WEBSOCKET | pull   | `/v1/stream/ws?subscription=X` — replay + live, acks  |

The filter is applied **once** at match time, so streams never re-evaluate it —
they just tail the subscription's delivery rows.

### Reliability

`DeliveryEvent` gets a monotonic `seq bigint` (identity) — the resumable
**cursor**. SSE/WS: on connect, replay rows with `seq > since`, then block on an
in-process **event hub** that is notified when a new row lands for the
subscription; on wake, flush newer rows and advance the cursor. Durable
(DB is truth) + real-time (hub wakeup) + resumable (cursor survives reconnects).
SSE uses the standard `Last-Event-ID` header; WS clients send `{ack: seq}`.

## Phase A components (this spec)

1. **Richer filter model** (`internal/pipeline/deliver.go` + `SignalForMatch`):
   fields `minSeverity`, `minConfidence`, `minRelevance`, `categories`/`tags`
   (hierarchical prefix match), `countries`, `regions`, `sentiment[]`,
   `minInfluence`, `entities[]`, `keyword` (title/summary substring). One
   `matchesFilter` powers all channels. `LoadSignalForMatch` extended to select
   relevance/region/sentiment/influence/title/summary/entities.
2. **Channels**: `ALTER TYPE "DeliveryChannel" ADD VALUE 'SSE','WEBSOCKET'`;
   both are pull-family (write delivery rows, no push worker action).
3. **Cursor**: `ALTER TABLE "DeliveryEvent" ADD COLUMN "seq" bigint identity` +
   index; `ListDeliveriesForStream(subID, sinceSeq, limit)` keyset query.
4. **Event hub** (`internal/stream`): `Hub` with `Notify(subID)` and
   `Subscribe(subID) -> <-chan struct{}` (coalescing, bounded). Fired from
   `MatchSubscriptions` after a pull-family row is created.
5. **Endpoints** (`internal/httpapi/stream_routes.go`): `GET /v1/stream/sse`
   and `/v1/stream/ws`, wrapped in `requireAPIKey("signals:read")`; resolve
   `?subscription=<id>`, replay-since-cursor then live. WebSocket via
   `github.com/coder/websocket` (small, stdlib-style).
6. **`example-clients/`** (`python/`, `typescript/`): `webhook_receiver`,
   `poll_client`, `sse_client`, `ws_client` + READMEs. An integration harness
   provisions an API key + subscription, emits a matching signal, and asserts
   each client receives it.

## Testing (target: exhaustive)

- **Unit**: `matchesFilter` every branch (pure); code-gen (Phase B); hub
  coalescing/close semantics.
- **API/integration** (real Postgres): `LoadSignalForMatch` fields; delivery-row
  cursor keyset; SSE endpoint (auth 401/403, replay, live push, `Last-Event-ID`
  resume); WS endpoint (subscribe, live push, ack/resume). Backend coverage gate
  ≥95% holds.
- **Example clients**: run against the live server in an integration harness
  asserting end-to-end receipt.
- **Browser**: a static demo page consumes SSE (fetch-stream with the auth
  header) and WS live, shown in-browser.

## Out of scope (Phase B)

The full-screen subscription modal, the visual filter builder, and the
in-app multi-language code-example generator.
