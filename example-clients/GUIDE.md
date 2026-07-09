# WorldSignal example clients — guide

This directory holds small, runnable clients that consume a WorldSignal
**subscription** over each of its delivery channels, in seven languages. They are
deliberately minimal — one file per (language × channel) — so you can read one
end to end and copy the ~30 lines you need into your own service.

The same code (byte for byte) is what the **Subscriptions** console generates in
its "Consume it" panel when you create a subscription, so what you see in the UI
is what you run here.

- [How delivery works](#how-delivery-works)
- [The four channels](#the-four-channels)
- [Configuration (env)](#configuration-env)
- [What each file does](#what-each-file-does)
- [Running by language](#running-by-language)
- [The signed-webhook contract](#the-signed-webhook-contract)
- [Automated test (`test.sh`)](#automated-test-testsh)

## How delivery works

When a signal matches a subscription's filter, the server writes a **delivery
row** to a durable, append-only log. Every row has a monotonic `seq` (a
`bigint` cursor). The transports below are just different ways of reading that
log:

- **Pull** transports (SSE, WebSocket, Polling) tail the log by cursor. You pass
  `?since=<seq>` and receive every row with a greater `seq`, oldest first. Persist
  the last `seq` you saw and you resume exactly where you left off — no gaps, no
  duplicates, even across restarts.
- **Push** transport (Webhook) POSTs each delivery to your URL as it happens, and
  signs the body so you can verify it came from WorldSignal.

Every event carries the same JSON envelope:

```jsonc
{
  "schema_version": "2026-06-01",
  "event_type": "signal.published",   // or "signal.test" for a test event
  "event_id": "cku…",
  "created_at": "2026-07-05T12:34:56.789Z",
  "subscription_id": "cmr…",
  "data": {                            // the signal itself
    "signal_id": "sg…", "title": "…", "summary": "…",
    "status": "CONFIRMED", "severity": "HIGH", "confidence": 0.9,
    "country": "US", "source_count": 12
  }
}
```

Each client reaches into `data` and prints `severity`, `country`, and `title`.
That is the only field access you need to understand every file here.

## The four channels

| Channel | Model | When to use | Auth |
|---------|-------|-------------|------|
| **SSE** | server holds one HTTP response open, streams `data:` lines | live dashboards, long-lived workers | header, or `?api_key=` in browsers |
| **WebSocket** | full-duplex socket, JSON frames `{seq, payload}` | live apps that also send messages upstream | header, or `?api_key=` in browsers |
| **Polling** | you `GET` on your own schedule, get events + next cursor | serverless / cron consumers, firewalled networks | header |
| **Webhook** | server POSTs signed events to your URL | event-driven backends, no inbound polling | HMAC signature you verify |

SSE and WebSocket give the lowest latency; if the connection drops you reconnect
with the last `seq`. Polling trades latency for operational simplicity (no held
connection). Webhook inverts control — the server calls you.

## Configuration (env)

Every pull client reads the same variables:

| Var | Default | Meaning |
|-----|---------|---------|
| `WS_API_BASE` | `http://localhost:4800` | server base URL |
| `WS_API_KEY` | — | API key with the `signals:read` scope (**required**) |
| `WS_SUBSCRIPTION` | `demo-stream` | subscription id to consume |
| `WS_SINCE` | `0` | resume cursor (`0` = from the start of the log) |
| `WS_MAX` | `0` | exit after N events (`0` = run forever) |
| `WS_INTERVAL` | `3` | seconds between polls (polling clients only) |

The webhook receivers instead read:

| Var | Default | Meaning |
|-----|---------|---------|
| `WS_WEBHOOK_SECRET` | — | must equal the server's `WEBHOOK_SIGNING_SECRET` |
| `WS_PORT` | `8088` | port the receiver listens on |
| `WS_MAX` | `0` | stop after N verified events |

Get a key and a subscription id from the **Subscriptions** console page (the
"Send test event" button there pushes one event so you can watch a client
receive it), or via the `createApiKey` / `createSubscription` GraphQL mutations.

## What each file does

Every language folder follows the same layout. Not every language ships every
channel — the pull channels (SSE/poll) are everywhere; WebSocket and the webhook
receiver are provided where the runtime has first-class support without extra
dependencies.

| File (per language dir) | Channel | What it does |
|-------------------------|---------|--------------|
| `sse_client.*` | SSE | Opens the stream with the `Authorization` header (not `EventSource`, which can't set headers), reads the response incrementally, and parses each `data:` line into an event. Exits after `WS_MAX`. |
| `poll_client.*` | Polling | Loops: `GET /v1/stream/poll?since=<cursor>`, prints each event in the batch, advances `cursor` to the returned value, sleeps `WS_INTERVAL`, repeats. The cursor is the whole story — persist it and you never miss or repeat an event. |
| `ws_client.*` | WebSocket | Connects to `/v1/stream/ws`, ignores the initial `{type:"connected"}` handshake frame, then prints each `{seq, payload}` frame. |
| `webhook_receiver.*` | Webhook | Runs a tiny HTTP server, recomputes the HMAC-SHA256 of the raw body with `WS_WEBHOOK_SECRET`, and rejects (`401`) anything whose `X-WorldSignal-Signature` doesn't match before trusting the payload. |

Coverage by language:

| Language | Dir | SSE | Poll | WebSocket | Webhook receiver |
|----------|-----|:---:|:----:|:---------:|:----------------:|
| Python | `python/` | ✅ | ✅ | ✅ (`websockets`) | ✅ |
| TypeScript | `typescript/` | ✅ | ✅ | ✅ (`ws`) | ✅ |
| Node.js | `node/` | ✅ | ✅ | ✅ (global `WebSocket`) | ✅ |
| Go | `go/` | ✅ | ✅ | — | — |
| Ruby | `ruby/` | ✅ | ✅ | — | — |
| PHP | `php/` | ✅ | ✅ | — | — |
| Shell (curl + jq) | `shell/` | ✅ | ✅ | — | — |
| Browser | `browser/index.html` | ✅ (via `EventSource` + `?api_key=`) | — | ✅ | — |

Go, Ruby, PHP and Shell ship the two pull channels using only their standard
library (plus `jq` for shell) — no package install. For WebSocket in those
languages, use the language's usual WebSocket library and the same
`/v1/stream/ws?subscription=…&since=…` URL; the SSE/poll files show the exact
envelope parsing you'll reuse.

## Running by language

```bash
# --- Python (SSE/poll/webhook = stdlib; WS needs `websockets`) --------------
pip install -r python/requirements.txt
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… python3 python/sse_client.py

# --- Node.js (21+ for the global WebSocket; otherwise dependency-free) ------
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… node node/sse_client.mjs

# --- TypeScript (Node 18+) --------------------------------------------------
cd typescript && npm install && WS_API_KEY=wsk_… npm run sse

# --- Go (stdlib only; each channel is its own package) ----------------------
cd go && WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… go run ./sse

# --- Ruby (stdlib only) -----------------------------------------------------
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… ruby ruby/sse_client.rb

# --- PHP (no extensions) ----------------------------------------------------
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… php php/sse_client.php

# --- Shell (curl + jq) ------------------------------------------------------
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… bash shell/sse_client.sh
```

Swap `sse` → `poll` (or `ws`, where present) for the other channels. The
polling clients accept `WS_INTERVAL=1` to poll faster.

## The signed-webhook contract

For `WEBHOOK` subscriptions the server POSTs the JSON envelope to your
`config.url` with header:

```
X-WorldSignal-Signature: sha256=<hex HMAC-SHA256 of the raw body, key = WEBHOOK_SIGNING_SECRET>
```

Verify it **before** parsing the body, comparing in constant time. The receivers
here do exactly that: recompute the digest, `timingSafeEqual` against the header,
return `401` on mismatch, and only then read `data`. Never trust an unsigned or
mismatched request.

## Automated test (`test.sh`)

`./test.sh` is the end-to-end proof that every client works against a **live**
server. It:

1. obtains a `signals:read` API key (admin login → `createApiKey`, unless you
   pass `WS_API_KEY`);
2. seeds a throwaway subscription whose filter matches nothing real, plus one
   known delivery carrying a unique marker string;
3. runs each client with `WS_SINCE=0 WS_MAX=1` and asserts it printed that exact
   marker;
4. for the webhook receivers, POSTs both a correctly-signed and a bad-signature
   request and asserts `200` vs `401`;
5. cleans up the subscription.

Each language block is guarded by a `command -v` check, so the script runs
whatever runtimes are installed and prints `… skipped` for the rest. Start the
server first (`./dev.sh`, or a `ROLE=all` binary), then:

```bash
cd example-clients && ./test.sh
```

A green run ends with `N passed, 0 failed`.
