import { createHash } from "node:crypto";

const STOPWORDS = new Set([
  "the", "a", "an", "and", "or", "but", "of", "to", "in", "on", "for", "with",
  "at", "by", "from", "as", "is", "are", "was", "were", "be", "been", "has",
  "have", "had", "it", "its", "this", "that", "these", "those", "after", "over",
  "into", "amid", "says", "said", "new", "report", "reports",
]);

export function stripHtml(html: string | undefined | null): string {
  if (!html) return "";
  return html
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&#39;|&apos;/gi, "'")
    .replace(/&quot;/gi, '"')
    .replace(/\s+/g, " ")
    .trim();
}

export function normalizeText(s: string): string {
  return s.toLowerCase().replace(/[^a-z0-9\s]/g, " ").replace(/\s+/g, " ").trim();
}

export function tokenSet(text: string): Set<string> {
  const norm = normalizeText(text);
  const tokens = norm.split(" ").filter((t) => t.length > 2 && !STOPWORDS.has(t));
  return new Set(tokens);
}

export function tokenSetString(text: string): string {
  return [...tokenSet(text)].sort().join(" ");
}

/** Jaccard similarity between two space-separated token-set strings. */
export function jaccard(aStr: string, bStr: string): number {
  const a = new Set(aStr.split(" ").filter(Boolean));
  const b = new Set(bStr.split(" ").filter(Boolean));
  if (a.size === 0 || b.size === 0) return 0;
  let inter = 0;
  for (const t of a) if (b.has(t)) inter++;
  const union = a.size + b.size - inter; // > 0 because both sets are non-empty here
  return inter / union;
}

export function contentHash(title: string, body: string): string {
  const basis = normalizeText(title) + "\n" + normalizeText(body).slice(0, 5000);
  return createHash("sha256").update(basis).digest("hex");
}

/** First N sentences — used by the heuristic summarizer fallback. */
export function firstSentences(text: string, n = 2): string {
  const clean = stripHtml(text);
  const sentences = clean.match(/[^.!?]+[.!?]+/g) ?? [clean];
  return sentences
    .slice(0, n)
    .map((s) => s.trim())
    .join(" ")
    .trim()
    .slice(0, 500);
}
