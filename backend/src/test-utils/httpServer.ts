import { createServer, type Server } from "node:http";

export interface TestServer {
  url: string;
  /** requests received, in order */
  requests: { headers: Record<string, string | string[] | undefined>; body: string }[];
  /** status code returned for POST requests (webhook delivery target) */
  setPostStatus: (status: number) => void;
  close: () => Promise<void>;
}

/** Start a throwaway HTTP server returning `respond(path)` for GET, capturing POSTs. */
export async function startTestServer(
  respond: (path: string) => { status?: number; contentType?: string; body: string },
): Promise<TestServer> {
  const requests: TestServer["requests"] = [];
  let postStatus = 200;
  const server: Server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (c) => chunks.push(c as Buffer));
    req.on("end", () => {
      const body = Buffer.concat(chunks).toString("utf8");
      if (req.method === "POST") {
        requests.push({ headers: req.headers, body });
        res.writeHead(postStatus).end(postStatus < 400 ? "ok" : "error");
        return;
      }
      const r = respond(req.url ?? "/");
      res.writeHead(r.status ?? 200, { "Content-Type": r.contentType ?? "application/xml" });
      res.end(r.body);
    });
  });

  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const addr = server.address();
  const port = typeof addr === "object" && addr ? addr.port : 0;
  return {
    url: `http://127.0.0.1:${port}`,
    requests,
    setPostStatus: (status: number) => {
      postStatus = status;
    },
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
  };
}

export const SAMPLE_RSS = `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Magnitude 6.8 earthquake hits Mindanao region</title>
      <link>https://example.com/quake?utm_source=x</link>
      <guid>guid-quake-1</guid>
      <pubDate>Mon, 15 Jun 2026 08:00:00 GMT</pubDate>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/"><![CDATA[<p>A strong earthquake of magnitude 6.8 struck near Mindanao. Authorities assess tsunami risk.</p>]]></content:encoded>
    </item>
    <item>
      <title>Central bank raises interest rates by 25 basis points</title>
      <link>https://example.com/rates</link>
      <guid>guid-rates-1</guid>
      <description>The central bank increased its benchmark interest rate citing inflation.</description>
    </item>
  </channel>
</rss>`;

export const SAMPLE_ATOM = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Test</title>
  <entry>
    <title>OpenAI unveils new AI model for developers</title>
    <link href="https://example.com/ai-model"/>
    <id>atom-ai-1</id>
    <updated>2026-06-15T09:00:00Z</updated>
    <summary>The company announced a new artificial intelligence model.</summary>
  </entry>
</feed>`;
