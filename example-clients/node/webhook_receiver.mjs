#!/usr/bin/env node
// Receive WorldSignal WEBHOOK deliveries and verify their HMAC signature.
//
// Point a WEBHOOK subscription's config.url at this server. Each POST carries an
// `X-WorldSignal-Signature: sha256=<hex>` header over the raw body, keyed by the
// deployment's WEBHOOK_SIGNING_SECRET. Dependency-free (node:http, node:crypto).
// Env: WS_WEBHOOK_SECRET (must match the server), WS_PORT (default 8088), WS_MAX.
import http from "node:http";
import crypto from "node:crypto";

const SECRET = process.env.WS_WEBHOOK_SECRET || "";
const PORT = Number(process.env.WS_PORT || "8088");
const MAX = Number(process.env.WS_MAX || "0");
let count = 0;

function validSignature(body, header) {
  if (!SECRET || !header) return false;
  const expected = "sha256=" + crypto.createHmac("sha256", SECRET).update(body).digest("hex");
  const a = Buffer.from(expected);
  const b = Buffer.from(header);
  return a.length === b.length && crypto.timingSafeEqual(a, b);
}

const srv = http.createServer((req, res) => {
  const chunks = [];
  req.on("data", (c) => chunks.push(c));
  req.on("end", () => {
    const body = Buffer.concat(chunks);
    const ok = validSignature(body, req.headers["x-worldsignal-signature"] || "");
    res.writeHead(ok ? 200 : 401).end();
    if (!ok) { console.error("[webhook] REJECTED (bad signature)"); return; }
    const d = JSON.parse(body.toString("utf8")).data || {};
    console.log(`[webhook] ${(d.severity || "?").padEnd(8)} ${d.country || "--"}  ${d.title || ""}`);
    if (MAX && ++count >= MAX) { srv.close(); }
  });
});
srv.listen(PORT, () => console.error(`[webhook] listening on :${PORT} (verifying signatures)`));
