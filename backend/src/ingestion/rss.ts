import Parser from "rss-parser";
import { stripHtml } from "../lib/text.js";

const parser = new Parser({
  timeout: 15000,
  headers: { "User-Agent": "WorldSignalBot/0.1 (+https://worldsignal.example/bot)" },
});

export interface DiscoveredItem {
  sourceGuid: string | null;
  url: string | null;
  title: string;
  content: string;
  author: string | null;
  publishedAt: Date | null;
  rawPayload: unknown;
}

/** First non-empty value from a list of candidate fields. */
function firstNonEmpty(...values: (string | undefined | null)[]): string | null {
  for (const v of values) {
    if (v != null && String(v).length > 0) return String(v);
  }
  return null;
}

export async function fetchRssSource(url: string): Promise<DiscoveredItem[]> {
  const feed = await parser.parseURL(url);
  const items: DiscoveredItem[] = [];
  for (const item of feed.items ?? []) {
    const title = (item.title ?? "").trim();
    if (!title) continue;
    const content = stripHtml(
      firstNonEmpty(
        item["content:encoded"] as string | undefined,
        item.content,
        item.contentSnippet,
        item.summary as string | undefined,
      ),
    );
    const published = firstNonEmpty(item.isoDate, item.pubDate);
    items.push({
      sourceGuid: firstNonEmpty(item.guid, item.id, item.link),
      url: firstNonEmpty(item.link),
      title,
      content,
      author: firstNonEmpty(item.creator, item.author as string | undefined),
      publishedAt: published ? new Date(published) : null,
      rawPayload: item,
    });
  }
  return items;
}
