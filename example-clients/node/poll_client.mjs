#!/usr/bin/env node
// Consume a WorldSignal subscription by polling — no connection held open.
// Each request returns events after ?since=<cursor> plus the next cursor; the
// client persists the cursor and polls again. Good for serverless / cron.
//
// Dependency-free (Node 18+ global fetch). Env: WS_API_BASE, WS_API_KEY,
// WS_SUBSCRIPTION, WS_SINCE, WS_MAX, WS_INTERVAL (seconds, default 3).
const BASE = process.env.WS_API_BASE || "http://localhost:4000";
const KEY = process.env.WS_API_KEY || "";
const SUB = process.env.WS_SUBSCRIPTION || "demo-stream";
const MAX = Number(process.env.WS_MAX || "0");
const INTERVAL = Number(process.env.WS_INTERVAL || "3");

const sleep = (s) => new Promise((r) => setTimeout(r, s * 1000));

async function main() {
  if (!KEY) { console.error("WS_API_KEY is required"); process.exit(2); }
  let cursor = Number(process.env.WS_SINCE || "0");
  let seen = 0;
  console.error(`[poll] ${BASE}/v1/stream/poll subscription=${SUB}`);
  for (;;) {
    const res = await fetch(`${BASE}/v1/stream/poll?subscription=${SUB}&since=${cursor}`,
      { headers: { Authorization: `Bearer ${KEY}` } });
    const body = await res.json();
    for (const ev of body.events || []) {
      const d = ev.payload?.data || {};
      console.log(`[poll] ${(d.severity || "?").padEnd(8)} ${d.country || "--"}  ${d.title || ""}`);
      if (MAX && ++seen >= MAX) return;
    }
    cursor = body.cursor ?? cursor;
    await sleep(INTERVAL);
  }
}
main().catch((e) => { console.error(e); process.exit(1); });
