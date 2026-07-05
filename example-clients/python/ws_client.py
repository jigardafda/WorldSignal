#!/usr/bin/env python3
"""Consume a WorldSignal subscription over a WebSocket.

Frames are {"seq": <n>, "payload": <envelope>}. Requires the `websockets`
package (see requirements.txt). Env: same as sse_client.py.
"""
import asyncio
import json
import os
import sys

import websockets

BASE = os.environ.get("WS_API_BASE", "http://localhost:4000")
KEY = os.environ.get("WS_API_KEY", "")
SUB = os.environ.get("WS_SUBSCRIPTION", "demo-stream")
SINCE = os.environ.get("WS_SINCE", "0")
MAX = int(os.environ.get("WS_MAX", "0"))


async def run() -> int:
    if not KEY:
        print("WS_API_KEY is required", file=sys.stderr)
        return 2
    url = BASE.replace("http://", "ws://").replace("https://", "wss://")
    url += f"/v1/stream/ws?subscription={SUB}&since={SINCE}"
    print(f"[ws] connecting to {url}", file=sys.stderr)

    seen = 0
    async with websockets.connect(url, additional_headers={"Authorization": f"Bearer {KEY}"}) as ws:
        async for raw in ws:
            msg = json.loads(raw)
            if msg.get("type") == "connected":
                print(f"[ws] connected to {msg.get('subscription')}", file=sys.stderr)
                continue
            d = msg.get("payload", {}).get("data", {})
            print(f"[ws] {d.get('severity','?'):8} {d.get('country') or '--'}  {d.get('title','')}")
            seen += 1
            if MAX and seen >= MAX:
                return 0
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(run()))
