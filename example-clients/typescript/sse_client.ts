/**
 * Consume a WorldSignal subscription as a Server-Sent Events stream.
 *
 * Env: WS_API_BASE (default http://localhost:4800), WS_API_KEY (required),
 *      WS_SUBSCRIPTION (default demo-stream), WS_SINCE (default 0), WS_MAX.
 *
 * Uses the built-in fetch stream — prefer the Authorization header over the
 * ?api_key= query param (browsers need it, but proxies may log URLs).
 */
const BASE = process.env.WS_API_BASE ?? "http://localhost:4800";
const KEY = process.env.WS_API_KEY ?? "";
const SUB = process.env.WS_SUBSCRIPTION ?? "demo-stream";
const SINCE = process.env.WS_SINCE ?? "0";
const MAX = Number(process.env.WS_MAX ?? "0");

interface SignalEvent { data?: { severity?: string; country?: string; title?: string } }

async function main(): Promise<void> {
  if (!KEY) { console.error("WS_API_KEY is required"); process.exit(2); }
  const url = `${BASE}/v1/stream/sse?subscription=${SUB}&since=${SINCE}`;
  console.error(`[sse] connecting to ${url}`);
  const resp = await fetch(url, { headers: { Authorization: `Bearer ${KEY}` } });
  if (!resp.ok || !resp.body) { console.error(`[sse] http ${resp.status}`); process.exit(1); }

  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let seen = 0;
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let nl: number;
    while ((nl = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, nl);
      buffer = buffer.slice(nl + 1);
      if (!line.startsWith("data:")) continue; // skip id:/event:/comments
      const event = JSON.parse(line.slice(5).trim()) as SignalEvent;
      const d = event.data ?? {};
      console.log(`[sse] ${(d.severity ?? "?").padEnd(8)} ${d.country ?? "--"}  ${d.title ?? ""}`);
      if (MAX && ++seen >= MAX) { console.error(`[sse] received ${seen}, exiting`); process.exit(0); }
    }
  }
}

void main();
