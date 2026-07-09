#!/usr/bin/env python3
"""Consume a WorldSignal subscription as a Server-Sent Events stream.

Env:
  WS_API_BASE     base URL (default http://localhost:4800)
  WS_API_KEY      API key with the signals:read scope (required)
  WS_SUBSCRIPTION subscription id (default demo-stream)
  WS_SINCE        resume cursor / seq (default: 0 = from the start)
  WS_MAX          exit after N events (default: 0 = run forever)

Uses only the standard library. Prefer the Authorization header (below) over
the ?api_key= query param, which browsers need but proxies may log.
"""
import json
import os
import sys
import urllib.request

BASE = os.environ.get("WS_API_BASE", "http://localhost:4800")
KEY = os.environ.get("WS_API_KEY", "")
SUB = os.environ.get("WS_SUBSCRIPTION", "demo-stream")
SINCE = os.environ.get("WS_SINCE", "0")
MAX = int(os.environ.get("WS_MAX", "0"))


def main() -> int:
    if not KEY:
        print("WS_API_KEY is required", file=sys.stderr)
        return 2
    url = f"{BASE}/v1/stream/sse?subscription={SUB}&since={SINCE}"
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {KEY}"})
    print(f"[sse] connecting to {url}", file=sys.stderr)

    seen = 0
    with urllib.request.urlopen(req) as resp:
        for raw in resp:
            line = raw.decode("utf-8", "replace").rstrip("\n")
            if not line.startswith("data:"):
                continue  # skip id:/event:/comment lines
            event = json.loads(line[len("data:"):].strip())
            d = event.get("data", {})
            print(f"[sse] {d.get('severity','?'):8} {d.get('country') or '--'}  {d.get('title','')}")
            seen += 1
            if MAX and seen >= MAX:
                print(f"[sse] received {seen} event(s), exiting", file=sys.stderr)
                return 0
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
