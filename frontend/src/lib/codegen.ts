// Generates copy-paste client snippets for consuming a subscription over each
// delivery channel, in the top languages. The subscription's filter is applied
// server-side, so a consumer only needs the base URL, the subscription id, and
// an API key (signals:read). Webhook snippets are receivers that verify the
// HMAC signature. Pure functions — unit-tested for structural invariants.

export type Channel = "WEBHOOK" | "EMAIL" | "POLLING" | "SSE" | "WEBSOCKET";
export type Language = "curl" | "node" | "typescript" | "python" | "go" | "php" | "ruby";

export const LANGUAGES: { id: Language; label: string }[] = [
  { id: "curl", label: "cURL" },
  { id: "node", label: "Node.js" },
  { id: "typescript", label: "TypeScript" },
  { id: "python", label: "Python" },
  { id: "go", label: "Go" },
  { id: "php", label: "PHP" },
  { id: "ruby", label: "Ruby" },
];

export interface CodeOpts {
  baseUrl: string;
  subscriptionId: string;
}

const KEY = "$WORLDSIGNAL_API_KEY"; // placeholder; a real key looks like wsk_…
const SECRET_ENV = "WORLDSIGNAL_WEBHOOK_SECRET";

function urls(o: CodeOpts) {
  const base = (o.baseUrl || "http://localhost:4800").replace(/\/+$/, "");
  const sub = encodeURIComponent(o.subscriptionId || "<subscription-id>");
  return {
    base,
    sub,
    sse: `${base}/v1/stream/sse?subscription=${sub}`,
    ws: `${base.replace(/^http/, "ws")}/v1/stream/ws?subscription=${sub}`,
    poll: `${base}/v1/stream/poll?subscription=${sub}`,
  };
}

/** codeFor returns a client snippet, or null when the channel needs no client
 * code (EMAIL is delivered to recipients). */
export function codeFor(language: Language, channel: Channel, o: CodeOpts): string | null {
  if (channel === "EMAIL") return null;
  const u = urls(o);
  const gen = GEN[language];
  return gen(channel, u);
}

type U = ReturnType<typeof urls>;
type Gen = (channel: Channel, u: U) => string;

const GEN: Record<Language, Gen> = {
  curl: (c, u) => {
    switch (c) {
      case "SSE":
        return `# Server-Sent Events — streams live, replays with ?since=<cursor>\ncurl -N -H "Authorization: Bearer ${KEY}" \\\n  "${u.sse}"`;
      case "POLLING":
        return `# Poll: pass the previous response's "cursor" as ?since to get only new events\ncurl -s -H "Authorization: Bearer ${KEY}" \\\n  "${u.poll}&since=0"`;
      case "WEBSOCKET":
        return `# WebSocket — cURL can't; use websocat (https://github.com/vi/websocat)\nwebsocat -H "Authorization: Bearer ${KEY}" \\\n  "${u.ws}"`;
      default: // WEBHOOK
        return `# Webhooks are POSTed to your server — see the Node/Python/Go tabs for a\n# receiver that verifies the X-WorldSignal-Signature header.`;
    }
  },

  node: (c, u) => {
    switch (c) {
      case "SSE":
        return `// Node 18+ (built-in fetch). Streams live; resume with ?since=<cursor>.
const res = await fetch("${u.sse}", {
  headers: { Authorization: \`Bearer \${process.env.WORLDSIGNAL_API_KEY}\` },
});
const reader = res.body.getReader();
const dec = new TextDecoder();
let buf = "";
for (;;) {
  const { value, done } = await reader.read();
  if (done) break;
  buf += dec.decode(value, { stream: true });
  let i;
  while ((i = buf.indexOf("\\n")) >= 0) {
    const line = buf.slice(0, i); buf = buf.slice(i + 1);
    if (line.startsWith("data:")) console.log(JSON.parse(line.slice(5)).data);
  }
}`;
      case "POLLING":
        return `// Node 18+. Persist \`cursor\` and poll again for only new events.
let cursor = 0;
for (;;) {
  const res = await fetch(\`${u.poll}&since=\${cursor}\`, {
    headers: { Authorization: \`Bearer \${process.env.WORLDSIGNAL_API_KEY}\` },
  });
  const { events, cursor: next } = await res.json();
  for (const e of events) console.log(e.payload.data);
  cursor = next;
  await new Promise((r) => setTimeout(r, 3000));
}`;
      case "WEBSOCKET":
        return `// npm i ws
import WebSocket from "ws";
const ws = new WebSocket("${u.ws}", {
  headers: { Authorization: \`Bearer \${process.env.WORLDSIGNAL_API_KEY}\` },
});
ws.on("message", (raw) => {
  const msg = JSON.parse(raw.toString());
  if (msg.payload) console.log(msg.payload.data);
});`;
      default: // WEBHOOK
        return `// Verify the HMAC signature over the raw body.
import { createHmac, timingSafeEqual } from "node:crypto";
import { createServer } from "node:http";
const SECRET = process.env.${SECRET_ENV};
createServer((req, res) => {
  const chunks = [];
  req.on("data", (c) => chunks.push(c));
  req.on("end", () => {
    const body = Buffer.concat(chunks);
    const expected = "sha256=" + createHmac("sha256", SECRET).update(body).digest("hex");
    const got = req.headers["x-worldsignal-signature"] || "";
    const ok = expected.length === got.length && timingSafeEqual(Buffer.from(expected), Buffer.from(got));
    res.writeHead(ok ? 200 : 401).end();
    if (ok) console.log(JSON.parse(body).data);
  });
}).listen(8088);`;
    }
  },

  typescript: (c, u) => GEN.node(c, u), // same runtime; the modal labels it TypeScript

  python: (c, u) => {
    switch (c) {
      case "SSE":
        return `# stdlib only. Streams live; resume with ?since=<cursor>.
import json, os, urllib.request
req = urllib.request.Request("${u.sse}",
    headers={"Authorization": f"Bearer {os.environ['WORLDSIGNAL_API_KEY']}"})
with urllib.request.urlopen(req) as resp:
    for raw in resp:
        line = raw.decode()
        if line.startswith("data:"):
            print(json.loads(line[5:])["data"])`;
      case "POLLING":
        return `# stdlib only. Persist \`cursor\` and poll again for only new events.
import json, os, time, urllib.request
cursor = 0
while True:
    req = urllib.request.Request(f"${u.poll}&since={cursor}",
        headers={"Authorization": f"Bearer {os.environ['WORLDSIGNAL_API_KEY']}"})
    with urllib.request.urlopen(req) as resp:
        body = json.load(resp)
    for e in body["events"]:
        print(e["payload"]["data"])
    cursor = body["cursor"]
    time.sleep(3)`;
      case "WEBSOCKET":
        return `# pip install websockets
import asyncio, json, os, websockets
async def main():
    async with websockets.connect("${u.ws}",
        additional_headers={"Authorization": f"Bearer {os.environ['WORLDSIGNAL_API_KEY']}"}) as ws:
        async for raw in ws:
            msg = json.loads(raw)
            if msg.get("payload"):
                print(msg["payload"]["data"])
asyncio.run(main())`;
      default: // WEBHOOK
        return `# stdlib only. Verify the HMAC signature over the raw body.
import hashlib, hmac, json, os
from http.server import BaseHTTPRequestHandler, HTTPServer
SECRET = os.environ["${SECRET_ENV}"].encode()
class H(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        expected = "sha256=" + hmac.new(SECRET, body, hashlib.sha256).hexdigest()
        ok = hmac.compare_digest(expected, self.headers.get("X-WorldSignal-Signature", ""))
        self.send_response(200 if ok else 401); self.end_headers()
        if ok: print(json.loads(body)["data"])
HTTPServer(("0.0.0.0", 8088), H).serve_forever()`;
    }
  },

  go: (c, u) => {
    switch (c) {
      case "SSE":
        return `// net/http + bufio. Streams live; resume with ?since=<cursor>.
req, _ := http.NewRequest("GET", "${u.sse}", nil)
req.Header.Set("Authorization", "Bearer "+os.Getenv("WORLDSIGNAL_API_KEY"))
resp, _ := http.DefaultClient.Do(req)
defer resp.Body.Close()
sc := bufio.NewScanner(resp.Body)
for sc.Scan() {
    if line := sc.Text(); strings.HasPrefix(line, "data:") {
        fmt.Println(line[5:])
    }
}`;
      case "POLLING":
        return `// Persist cursor and poll again for only new events.
cursor := 0
for {
    url := fmt.Sprintf("${u.poll}&since=%d", cursor)
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+os.Getenv("WORLDSIGNAL_API_KEY"))
    resp, _ := http.DefaultClient.Do(req)
    var body struct {
        Cursor int \`json:"cursor"\`
        Events []struct{ Payload json.RawMessage \`json:"payload"\` } \`json:"events"\`
    }
    json.NewDecoder(resp.Body).Decode(&body)
    resp.Body.Close()
    for _, e := range body.Events { fmt.Println(string(e.Payload)) }
    cursor = body.Cursor
    time.Sleep(3 * time.Second)
}`;
      case "WEBSOCKET":
        return `// go get github.com/coder/websocket
ctx := context.Background()
c, _, _ := websocket.Dial(ctx, "${u.ws}", &websocket.DialOptions{
    HTTPHeader: http.Header{"Authorization": {"Bearer " + os.Getenv("WORLDSIGNAL_API_KEY")}},
})
defer c.CloseNow()
for {
    _, data, err := c.Read(ctx)
    if err != nil { break }
    fmt.Println(string(data))
}`;
      default: // WEBHOOK
        return `// Verify the HMAC signature over the raw body.
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    mac := hmac.New(sha256.New, []byte(os.Getenv("${SECRET_ENV}")))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    if hmac.Equal([]byte(expected), []byte(r.Header.Get("X-WorldSignal-Signature"))) {
        w.WriteHeader(200); fmt.Println(string(body))
    } else {
        w.WriteHeader(401)
    }
})
http.ListenAndServe(":8088", nil)`;
    }
  },

  php: (c, u) => {
    switch (c) {
      case "SSE":
        return `<?php // Streams live; resume with ?since=<cursor>.
$ctx = stream_context_create(["http" => ["header" =>
  "Authorization: Bearer " . getenv("WORLDSIGNAL_API_KEY")]]);
$fh = fopen("${u.sse}", "r", false, $ctx);
while (!feof($fh)) {
  $line = fgets($fh);
  if (str_starts_with($line, "data:")) {
    print_r(json_decode(substr($line, 5), true)["data"]);
  }
}`;
      case "POLLING":
        return `<?php // Persist $cursor and poll again for only new events.
$cursor = 0;
while (true) {
  $ctx = stream_context_create(["http" => ["header" =>
    "Authorization: Bearer " . getenv("WORLDSIGNAL_API_KEY")]]);
  $body = json_decode(file_get_contents("${u.poll}&since=$cursor", false, $ctx), true);
  foreach ($body["events"] as $e) { print_r($e["payload"]["data"]); }
  $cursor = $body["cursor"];
  sleep(3);
}`;
      case "WEBSOCKET":
        return `<?php // composer require textalk/websocket
$client = new WebSocket\\Client("${u.ws}", ["headers" =>
  ["Authorization" => "Bearer " . getenv("WORLDSIGNAL_API_KEY")]]);
while (true) {
  $msg = json_decode($client->receive(), true);
  if (isset($msg["payload"])) { print_r($msg["payload"]["data"]); }
}`;
      default: // WEBHOOK
        return `<?php // Verify the HMAC signature over the raw body.
$body = file_get_contents("php://input");
$expected = "sha256=" . hash_hmac("sha256", $body, getenv("${SECRET_ENV}"));
$got = $_SERVER["HTTP_X_WORLDSIGNAL_SIGNATURE"] ?? "";
if (hash_equals($expected, $got)) {
  http_response_code(200);
  print_r(json_decode($body, true)["data"]);
} else {
  http_response_code(401);
}`;
    }
  },

  ruby: (c, u) => {
    switch (c) {
      case "SSE":
        return `require "net/http"; require "json"
uri = URI("${u.sse}")
Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https") do |http|
  req = Net::HTTP::Get.new(uri)
  req["Authorization"] = "Bearer #{ENV['WORLDSIGNAL_API_KEY']}"
  http.request(req) do |res|
    res.read_body do |chunk|
      chunk.each_line do |line|
        puts JSON.parse(line[5..])["data"] if line.start_with?("data:")
      end
    end
  end
end`;
      case "POLLING":
        return `require "net/http"; require "json"
cursor = 0
loop do
  uri = URI("${u.poll}&since=#{cursor}")
  req = Net::HTTP::Get.new(uri)
  req["Authorization"] = "Bearer #{ENV['WORLDSIGNAL_API_KEY']}"
  res = Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https") { |h| h.request(req) }
  body = JSON.parse(res.body)
  body["events"].each { |e| puts e["payload"]["data"] }
  cursor = body["cursor"]
  sleep 3
end`;
      case "WEBSOCKET":
        return `# gem install websocket-client-simple
require "websocket-client-simple"; require "json"
ws = WebSocket::Client::Simple.connect("${u.ws}",
  headers: { "Authorization" => "Bearer #{ENV['WORLDSIGNAL_API_KEY']}" })
ws.on(:message) do |msg|
  data = JSON.parse(msg.data)
  puts data["payload"]["data"] if data["payload"]
end
sleep`;
      default: // WEBHOOK
        return `require "openssl"; require "json"
# In your Rack/Rails handler, with \`body\` = the raw request body:
expected = "sha256=" + OpenSSL::HMAC.hexdigest("SHA256", ENV["${SECRET_ENV}"], body)
if Rack::Utils.secure_compare(expected, request.env["HTTP_X_WORLDSIGNAL_SIGNATURE"].to_s)
  puts JSON.parse(body)["data"]
end`;
    }
  },
};
