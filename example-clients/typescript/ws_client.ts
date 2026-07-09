/**
 * Consume a WorldSignal subscription over a WebSocket.
 * Frames are {"seq": n, "payload": <envelope>}. Requires the `ws` package.
 * Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX.
 */
import WebSocket from "ws";

const BASE = process.env.WS_API_BASE ?? "http://localhost:4800";
const KEY = process.env.WS_API_KEY ?? "";
const SUB = process.env.WS_SUBSCRIPTION ?? "demo-stream";
const SINCE = process.env.WS_SINCE ?? "0";
const MAX = Number(process.env.WS_MAX ?? "0");

if (!KEY) { console.error("WS_API_KEY is required"); process.exit(2); }

const url = BASE.replace(/^http/, "ws") + `/v1/stream/ws?subscription=${SUB}&since=${SINCE}`;
console.error(`[ws] connecting to ${url}`);
const ws = new WebSocket(url, { headers: { Authorization: `Bearer ${KEY}` } });

let seen = 0;
ws.on("message", (raw: Buffer) => {
  const msg = JSON.parse(raw.toString()) as { type?: string; subscription?: string; payload?: { data?: { severity?: string; country?: string; title?: string } } };
  if (msg.type === "connected") { console.error(`[ws] connected to ${msg.subscription}`); return; }
  const d = msg.payload?.data ?? {};
  console.log(`[ws] ${(d.severity ?? "?").padEnd(8)} ${d.country ?? "--"}  ${d.title ?? ""}`);
  if (MAX && ++seen >= MAX) { ws.close(); process.exit(0); }
});
ws.on("error", (err: Error) => { console.error(`[ws] error: ${err.message}`); process.exit(1); });
