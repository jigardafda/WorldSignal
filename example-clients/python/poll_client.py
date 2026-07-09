#!/usr/bin/env python3
"""Consume a WorldSignal subscription by polling — no connection held open.

Each request returns the events after ?since=<cursor> plus the next cursor; the
client persists the cursor and polls again. Good for serverless / cron consumers.

Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX (see sse_client.py),
     WS_INTERVAL seconds between polls (default 3).
"""
import json
import os
import sys
import time
import urllib.request

BASE = os.environ.get("WS_API_BASE", "http://localhost:4800")
KEY = os.environ.get("WS_API_KEY", "")
SUB = os.environ.get("WS_SUBSCRIPTION", "demo-stream")
MAX = int(os.environ.get("WS_MAX", "0"))
INTERVAL = float(os.environ.get("WS_INTERVAL", "3"))


def poll(cursor: int):
    url = f"{BASE}/v1/stream/poll?subscription={SUB}&since={cursor}"
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {KEY}"})
    with urllib.request.urlopen(req) as resp:
        return json.load(resp)


def main() -> int:
    if not KEY:
        print("WS_API_KEY is required", file=sys.stderr)
        return 2
    cursor = int(os.environ.get("WS_SINCE", "0"))
    seen = 0
    print(f"[poll] {BASE}/v1/stream/poll subscription={SUB}", file=sys.stderr)
    while True:
        body = poll(cursor)
        for ev in body.get("events", []):
            d = ev["payload"].get("data", {})
            print(f"[poll] {d.get('severity','?'):8} {d.get('country') or '--'}  {d.get('title','')}")
            seen += 1
            if MAX and seen >= MAX:
                return 0
        cursor = body.get("cursor", cursor)
        time.sleep(INTERVAL)


if __name__ == "__main__":
    raise SystemExit(main())
