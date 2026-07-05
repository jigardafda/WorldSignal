#!/usr/bin/env python3
"""Receive WorldSignal WEBHOOK deliveries and verify their HMAC signature.

Point a WEBHOOK subscription's config.url at this server. Each POST carries an
`X-WorldSignal-Signature: sha256=<hex>` header over the raw body, keyed by the
deployment's WEBHOOK_SIGNING_SECRET.

Env: WS_WEBHOOK_SECRET (must match the server), WS_PORT (default 8088), WS_MAX.
Uses only the standard library.
"""
import hashlib
import hmac
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

SECRET = os.environ.get("WS_WEBHOOK_SECRET", "").encode()
PORT = int(os.environ.get("WS_PORT", "8088"))
MAX = int(os.environ.get("WS_MAX", "0"))
_count = {"n": 0}


def valid_signature(body: bytes, header: str) -> bool:
    if not SECRET or not header:
        return False
    expected = "sha256=" + hmac.new(SECRET, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, header)


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        ok = valid_signature(body, self.headers.get("X-WorldSignal-Signature", ""))
        self.send_response(200 if ok else 401)
        self.end_headers()
        if not ok:
            print("[webhook] REJECTED (bad signature)", file=sys.stderr)
            return
        d = json.loads(body).get("data", {})
        print(f"[webhook] {d.get('severity','?'):8} {d.get('country') or '--'}  {d.get('title','')}")
        _count["n"] += 1
        if MAX and _count["n"] >= MAX:
            raise KeyboardInterrupt

    def log_message(self, *_):  # silence default access logging
        pass


def main() -> int:
    srv = HTTPServer(("0.0.0.0", PORT), Handler)
    print(f"[webhook] listening on :{PORT} (verifying signatures)", file=sys.stderr)
    try:
        srv.serve_forever()
    except KeyboardInterrupt:
        pass
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
