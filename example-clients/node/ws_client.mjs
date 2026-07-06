#!/usr/bin/env node
// Consume a WorldSignal subscription over a WebSocket using Node's built-in
// global WebSocket (Node 21+/22). That client is browser-compatible and can't
// set request headers, so it authenticates with the ?api_key= query param
// (the same fallback the browser uses). Frames are {seq, payload} JSON, with a
// leading {type:"connected"} handshake. Env: same as sse_client.mjs.
const BASE = process.env.WS_API_BASE || "http://localhost:4000";
const KEY = process.env.WS_API_KEY || "";
const SUB = process.env.WS_SUBSCRIPTION || "demo-stream";
const SINCE = process.env.WS_SINCE || "0";
const MAX = Number(process.env.WS_MAX || "0");

if (!KEY) { console.error("WS_API_KEY is required"); process.exit(2); }
const url = BASE.replace(/^http/, "ws") +
  `/v1/stream/ws?subscription=${SUB}&since=${SINCE}&api_key=${encodeURIComponent(KEY)}`;
console.error(`[ws] connecting to ${BASE.replace(/^http/, "ws")}/v1/stream/ws`);

let seen = 0;
const ws = new WebSocket(url);
ws.addEventListener("message", (e) => {
  const msg = JSON.parse(e.data);
  if (msg.type === "connected") { console.error(`[ws] connected to ${msg.subscription}`); return; }
  const d = msg.payload?.data || {};
  console.log(`[ws] ${(d.severity || "?").padEnd(8)} ${d.country || "--"}  ${d.title || ""}`);
  if (MAX && ++seen >= MAX) { ws.close(); process.exit(0); }
});
ws.addEventListener("error", (e) => { console.error("[ws] error", e.message || e); process.exit(1); });
