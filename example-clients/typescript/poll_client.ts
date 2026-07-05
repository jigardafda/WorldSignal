/**
 * Consume a WorldSignal subscription by polling — no connection held open.
 * Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX, WS_INTERVAL.
 */
const BASE = process.env.WS_API_BASE ?? "http://localhost:4000";
const KEY = process.env.WS_API_KEY ?? "";
const SUB = process.env.WS_SUBSCRIPTION ?? "demo-stream";
const MAX = Number(process.env.WS_MAX ?? "0");
const INTERVAL = Number(process.env.WS_INTERVAL ?? "3") * 1000;

interface PollResponse { cursor: number; events: { seq: number; payload: { data?: { severity?: string; country?: string; title?: string } } }[] }

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function main(): Promise<void> {
  if (!KEY) { console.error("WS_API_KEY is required"); process.exit(2); }
  let cursor = Number(process.env.WS_SINCE ?? "0");
  let seen = 0;
  console.error(`[poll] ${BASE}/v1/stream/poll subscription=${SUB}`);
  for (;;) {
    const resp = await fetch(`${BASE}/v1/stream/poll?subscription=${SUB}&since=${cursor}`, {
      headers: { Authorization: `Bearer ${KEY}` },
    });
    if (!resp.ok) { console.error(`[poll] http ${resp.status}`); process.exit(1); }
    const body = (await resp.json()) as PollResponse;
    for (const ev of body.events) {
      const d = ev.payload.data ?? {};
      console.log(`[poll] ${(d.severity ?? "?").padEnd(8)} ${d.country ?? "--"}  ${d.title ?? ""}`);
      if (MAX && ++seen >= MAX) process.exit(0);
    }
    cursor = body.cursor ?? cursor;
    await sleep(INTERVAL);
  }
}

void main();
