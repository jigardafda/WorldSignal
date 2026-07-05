# WorldSignal example clients

Runnable, minimal clients for every WorldSignal subscription delivery channel, in
**seven languages**. Point them at a running server and a subscription id. Each
file is the same code the **Subscriptions** console shows in its "Consume it"
panel, so what you copy from the UI is what runs here.

**👉 See [GUIDE.md](GUIDE.md) for a full walkthrough** of how delivery works, what
every file does, and how to run each language.

| Channel   | What it is                      | Languages |
|-----------|---------------------------------|-----------|
| SSE       | live stream, replay + resume    | Python · TypeScript · Node.js · Go · Ruby · PHP · Shell · Browser |
| WebSocket | live stream, bidirectional      | Python · TypeScript · Node.js · Browser |
| Polling   | cursor pull, no open connection | Python · TypeScript · Node.js · Go · Ruby · PHP · Shell |
| Webhook   | receive + verify signed POSTs   | Python · TypeScript · Node.js |

Go, Ruby, PHP and Shell use only their standard library (Shell needs `jq`); no
package install. Directory layout is identical per language: `sse_client.*`,
`poll_client.*`, and where present `ws_client.*` / `webhook_receiver.*`.

## Configure (env)

| Var | Default | Meaning |
|-----|---------|---------|
| `WS_API_BASE` | `http://localhost:4000` | server base URL |
| `WS_API_KEY` | — | API key with the `signals:read` scope (required for SSE/WS/poll) |
| `WS_SUBSCRIPTION` | `demo-stream` | subscription id to consume |
| `WS_SINCE` | `0` | resume cursor (`0` = from the start) |
| `WS_MAX` | `0` | exit after N events (`0` = run forever) |
| `WS_INTERVAL` | `3` | seconds between polls (polling clients) |
| `WS_WEBHOOK_SECRET` | — | must equal the server's `WEBHOOK_SIGNING_SECRET` (webhook receiver) |

Get a key + subscription from the **Subscriptions** console page (its "Send test
event" button pushes one event so you can watch a client receive it), or via the
`createApiKey` / `createSubscription` GraphQL mutations.

## Run

```bash
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… python3 python/sse_client.py   # Python
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… node    node/sse_client.mjs    # Node.js
( cd typescript && npm install && WS_API_KEY=wsk_… npm run sse )      # TypeScript
( cd go && WS_API_KEY=wsk_… go run ./sse )                            # Go
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… ruby    ruby/sse_client.rb      # Ruby
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… php     php/sse_client.php      # PHP
WS_API_KEY=wsk_… WS_SUBSCRIPTION=cmr… bash    shell/sse_client.sh     # Shell
```

Swap `sse` → `poll` (or `ws`, where present). See [GUIDE.md](GUIDE.md) for details.

## Auth note

Server clients send `Authorization: Bearer <key>`. Browsers can't set headers on
`EventSource`/`WebSocket`, so the stream routes also accept `?api_key=<key>` —
prefer the header where you can, since proxies/history may retain URLs.

## Test

`./test.sh` provisions a throwaway subscription, injects one known delivery, and
asserts every client (in every installed language) receives it against a live
server (`WS_API_BASE`). Run the server first (`./dev.sh` or a `ROLE=all` binary).
Languages whose runtime isn't installed are skipped. A green run ends with
`N passed, 0 failed`.
