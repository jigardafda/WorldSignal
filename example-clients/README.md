# WorldSignal example clients

Runnable, minimal clients for every WorldSignal subscription delivery channel, in
**Python** and **TypeScript**. Point them at a running server and a subscription.

| Channel   | What it is                        | Python                 | TypeScript             |
|-----------|-----------------------------------|------------------------|------------------------|
| SSE       | live stream, replay + resume      | `python/sse_client.py` | `typescript/sse_client.ts` |
| WebSocket | live stream, bidirectional        | `python/ws_client.py`  | `typescript/ws_client.ts`  |
| Polling   | cursor pull, no open connection   | `python/poll_client.py`| `typescript/poll_client.ts`|
| Webhook   | receive + verify signed POSTs     | `python/webhook_receiver.py` | `typescript/webhook_receiver.ts` |

## Configure (env)

| Var | Default | Meaning |
|-----|---------|---------|
| `WS_API_BASE` | `http://localhost:4000` | server base URL |
| `WS_API_KEY` | — | API key with the `signals:read` scope (required for SSE/WS/poll) |
| `WS_SUBSCRIPTION` | `demo-stream` | subscription id to consume |
| `WS_SINCE` | `0` | resume cursor (`0` = from the start) |
| `WS_MAX` | `0` | exit after N events (`0` = run forever) |
| `WS_WEBHOOK_SECRET` | — | must equal the server's `WEBHOOK_SIGNING_SECRET` (webhook receiver) |

Get a key + subscription from the **Subscriptions** console page, or via GraphQL
(`createApiKey`, `createSubscription`).

## Run

```bash
# Python (SSE/poll/webhook use only the stdlib; WS needs `websockets`)
pip install -r python/requirements.txt
WS_API_KEY=wsk_... python3 python/sse_client.py

# TypeScript (Node 18+)
cd typescript && npm install
WS_API_KEY=wsk_... npm run sse
```

## Auth note

Server clients send `Authorization: Bearer <key>`. Browsers can't set headers on
`EventSource`/`WebSocket`, so the stream routes also accept `?api_key=<key>` —
prefer the header where you can, since proxies/history may retain URLs.

## Test

`./test.sh` provisions a throwaway subscription, injects one known delivery, and
asserts every client receives it against a live server (`WS_API_BASE`). Run the
server first (`./dev.sh` or `ROLE=all` binary).
