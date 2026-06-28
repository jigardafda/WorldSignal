import "../test-utils/env.js";
import { test, after } from "node:test";
import assert from "node:assert/strict";
import { fetchRssSource } from "./rss.js";
import { startTestServer, SAMPLE_RSS, SAMPLE_ATOM, type TestServer } from "../test-utils/httpServer.js";

const servers: TestServer[] = [];
async function serve(body: string) {
  const s = await startTestServer(() => ({ body }));
  servers.push(s);
  return s.url;
}
after(async () => {
  for (const s of servers) await s.close();
});

test("parses RSS items, extracting content from varying fields", async () => {
  const items = await fetchRssSource(await serve(SAMPLE_RSS));
  assert.equal(items.length, 2);
  const quake = items.find((i) => i.title.includes("earthquake"))!;
  assert.ok(quake.content.includes("magnitude"));
  assert.equal(quake.sourceGuid, "guid-quake-1");
  assert.ok(quake.publishedAt instanceof Date);
  const rates = items.find((i) => i.title.includes("interest"))!;
  assert.ok(rates.content.includes("benchmark"));
});

test("parses Atom feeds", async () => {
  const items = await fetchRssSource(await serve(SAMPLE_ATOM));
  assert.equal(items.length, 1);
  assert.ok(items[0].title.includes("AI model"));
  assert.equal(items[0].url, "https://example.com/ai-model");
});

test("falls back through content, guid and url fields", async () => {
  // item A: no content at all, guid from <link>, author present
  // item B: empty fields → sourceGuid null, url null, content empty
  const xml = `<?xml version="1.0"?><rss version="2.0"><channel><title>F</title>
    <item>
      <title>Author item with link guid</title>
      <link>https://x.com/a</link>
      <author>jane@news.com</author>
    </item>
    <item>
      <title>Bare item no link no guid no body</title>
    </item>
  </channel></rss>`;
  const items = await fetchRssSource(await serve(xml));
  const a = items.find((i) => i.title.includes("Author"))!;
  assert.equal(a.sourceGuid, "https://x.com/a"); // guid fell back to link
  assert.ok(a.author && a.author.includes("jane"));
  const b = items.find((i) => i.title.includes("Bare"))!;
  assert.equal(b.sourceGuid, null);
  assert.equal(b.url, null);
  assert.equal(b.content, "");
  assert.equal(b.publishedAt, null);
});

test("skips items without a title", async () => {
  const xml = `<?xml version="1.0"?><rss version="2.0"><channel><title>F</title>
    <item><link>https://x.com/a</link><guid>g1</guid></item>
    <item><title>Has a title</title><link>https://x.com/b</link><guid>g2</guid></item>
  </channel></rss>`;
  const items = await fetchRssSource(await serve(xml));
  assert.equal(items.length, 1);
  assert.equal(items[0].title, "Has a title");
});
