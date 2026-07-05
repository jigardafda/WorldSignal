/**
 * Receive WorldSignal WEBHOOK deliveries and verify their HMAC signature.
 * Env: WS_WEBHOOK_SECRET (must match the server), WS_PORT (default 8088), WS_MAX.
 */
import { createHmac, timingSafeEqual } from "node:crypto";
import { createServer } from "node:http";

const SECRET = process.env.WS_WEBHOOK_SECRET ?? "";
const PORT = Number(process.env.WS_PORT ?? "8088");
const MAX = Number(process.env.WS_MAX ?? "0");

function validSignature(body: Buffer, header: string | undefined): boolean {
  if (!SECRET || !header) return false;
  const expected = "sha256=" + createHmac("sha256", SECRET).update(body).digest("hex");
  const a = Buffer.from(expected);
  const b = Buffer.from(header);
  return a.length === b.length && timingSafeEqual(a, b);
}

let count = 0;
const server = createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on("data", (c: Buffer) => chunks.push(c));
  req.on("end", () => {
    const body = Buffer.concat(chunks);
    if (!validSignature(body, req.headers["x-worldsignal-signature"] as string | undefined)) {
      res.writeHead(401).end();
      console.error("[webhook] REJECTED (bad signature)");
      return;
    }
    res.writeHead(200).end();
    const d = (JSON.parse(body.toString()).data ?? {}) as { severity?: string; country?: string; title?: string };
    console.log(`[webhook] ${(d.severity ?? "?").padEnd(8)} ${d.country ?? "--"}  ${d.title ?? ""}`);
    if (MAX && ++count >= MAX) { server.close(); process.exit(0); }
  });
});
server.listen(PORT, () => console.error(`[webhook] listening on :${PORT} (verifying signatures)`));
