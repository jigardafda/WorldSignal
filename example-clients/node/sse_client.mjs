#!/usr/bin/env node
// Consume a WorldSignal subscription as Server-Sent Events.
//
// The browser EventSource API can't set an Authorization header, so this reads
// the stream with fetch() and parses `data:` lines itself — letting it use the
// header (preferred) instead of the ?api_key= query param. Dependency-free.
// Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX.
const BASE = process.env.WS_API_BASE || "http://localhost:4000";
const KEY = process.env.WS_API_KEY || "";
const SUB = process.env.WS_SUBSCRIPTION || "demo-stream";
const SINCE = process.env.WS_SINCE || "0";
const MAX = Number(process.env.WS_MAX || "0");

async function main() {
  if (!KEY) { console.error("WS_API_KEY is required"); process.exit(2); }
  const url = `${BASE}/v1/stream/sse?subscription=${SUB}&since=${SINCE}`;
  console.error(`[sse] connecting to ${url}`);
  const res = await fetch(url, { headers: { Authorization: `Bearer ${KEY}` } });
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  let seen = 0;
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    let nl;
    while ((nl = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, nl).replace(/\r$/, "");
      buf = buf.slice(nl + 1);
      if (!line.startsWith("data:")) continue; // skip id:/event:/comments
      const ev = JSON.parse(line.slice(5).trim());
      const d = ev.data || {};
      console.log(`[sse] ${(d.severity || "?").padEnd(8)} ${d.country || "--"}  ${d.title || ""}`);
      if (MAX && ++seen >= MAX) { console.error(`[sse] received ${seen} event(s), exiting`); return; }
    }
  }
}
main().catch((e) => { console.error(e); process.exit(1); });
