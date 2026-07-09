// Pure presentation helpers for the "For You" relevance feed. Kept out of the
// components so ranking, score bands, reason labels and age formatting are unit
// tested and used consistently across the page.
import { categoryLabel, domainOf } from "./categories";
import type { FeedItem } from "./api";

/** A relevance band drives the score ring's color and its short label. */
export interface ScoreBand {
  color: string; // Mantine color key
  label: string; // human tier
}

/** scoreBand maps a 0–10 relevance score to a tier. Strong matches read as the
 * brand accent; weak ones recede to grey so the eye lands on what matters. */
export function scoreBand(score: number): ScoreBand {
  if (score >= 7) return { color: "indigo", label: "Strong match" };
  if (score >= 4) return { color: "blue", label: "Relevant" };
  if (score >= 2) return { color: "cyan", label: "Loosely related" };
  return { color: "gray", label: "Background" };
}

/** scorePct clamps a score to the 0–100 the ring expects. */
export function scorePct(score: number): number {
  return Math.max(0, Math.min(100, Math.round(score * 10)));
}

/** A reason chip, humanized from a raw interest key like "entity:Marcus Vale". */
export interface ReasonChip {
  label: string;
  kind: "entity" | "topic" | "place" | "keyword" | "sentiment" | "other";
}

const SENTIMENT_LABEL: Record<string, string> = {
  POSITIVE: "Positive sentiment",
  NEGATIVE: "Negative sentiment",
  NEUTRAL: "Neutral sentiment",
};

/** humanizeReason turns a matched interest key into a readable chip. */
export function humanizeReason(key: string): ReasonChip {
  const i = key.indexOf(":");
  if (i < 0) return { label: key, kind: "other" };
  const dim = key.slice(0, i);
  const val = key.slice(i + 1);
  switch (dim) {
    case "entity":
      return { label: val, kind: "entity" };
    case "tag":
      return { label: categoryLabel(domainOf(val)) || val, kind: "topic" };
    case "country":
      return { label: val, kind: "place" };
    case "region":
      return { label: val, kind: "place" };
    case "keyword":
      return { label: `“${val}”`, kind: "keyword" };
    case "sentiment":
      return { label: SENTIMENT_LABEL[val.toUpperCase()] ?? val, kind: "sentiment" };
    default:
      return { label: val, kind: "other" };
  }
}

const REASON_COLOR: Record<ReasonChip["kind"], string> = {
  entity: "grape",
  topic: "blue",
  place: "teal",
  keyword: "orange",
  sentiment: "pink",
  other: "gray",
};

export function reasonColor(kind: ReasonChip["kind"]): string {
  return REASON_COLOR[kind];
}

/** formatAge renders hours-since as a compact relative time. */
export function formatAge(hours: number): string {
  if (hours < 0) hours = 0;
  if (hours < 1) {
    const m = Math.max(1, Math.round(hours * 60));
    return `${m}m ago`;
  }
  if (hours < 24) return `${Math.round(hours)}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

export type RankMode = "relevance" | "recency";

/** rankItems returns a new array ordered by the chosen mode. The server already
 * ranks by relevance; recency re-sorts client-side by age. */
export function rankItems(items: FeedItem[], mode: RankMode): FeedItem[] {
  const out = [...items];
  if (mode === "recency") out.sort((a, b) => a.ageHours - b.ageHours);
  else out.sort((a, b) => b.score - a.score);
  return out;
}
