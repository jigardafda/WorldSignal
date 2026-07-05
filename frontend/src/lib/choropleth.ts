// Pure aggregation + color-scale helpers for the choropleth ("Regions") view.
// Countries are colored by a metric computed from the visible markers:
//   - count / severity  → sequential (one hue, light→dark)
//   - sentiment         → diverging (red ↔ neutral gray ↔ green)
// Kept dependency-free (aside from the shared severity rank) and unit-tested.

import { severityRank } from "./liveMarkers";

export type Metric = "count" | "severity" | "sentiment";

/** Per-country tallies gathered in one pass over the markers. */
export interface CountryAgg {
  count: number;
  hi: number; // HIGH/CRITICAL count
  pos: number; // POSITIVE sentiment count
  neg: number; // NEGATIVE sentiment count
}

interface Countryish {
  country?: string | null;
  severity?: string | null;
  sentiment?: string | null;
}

/** Tally markers by ISO country code. */
export function aggregateByCountry(markers: Countryish[]): Map<string, CountryAgg> {
  const out = new Map<string, CountryAgg>();
  for (const m of markers) {
    if (!m.country) continue;
    const a = out.get(m.country) ?? { count: 0, hi: 0, pos: 0, neg: 0 };
    a.count++;
    if (severityRank(m.severity) >= 2) a.hi++;
    if (m.sentiment === "POSITIVE") a.pos++;
    else if (m.sentiment === "NEGATIVE") a.neg++;
    out.set(m.country, a);
  }
  return out;
}

/** The metric's value for a country: count (n), severity (HIGH share 0..1),
 * or sentiment (net (pos−neg)/count in [-1,1]). */
export function metricValue(a: CountryAgg, metric: Metric): number {
  if (metric === "count") return a.count;
  if (!a.count) return 0;
  if (metric === "severity") return a.hi / a.count;
  return (a.pos - a.neg) / a.count;
}

/** Domain max used to normalize the sequential COUNT scale (≥1). Severity and
 * sentiment have fixed domains (0..1 and [-1,1]) so this is only meaningful for
 * count. */
export function metricMax(aggs: Iterable<CountryAgg>, metric: Metric): number {
  if (metric !== "count") return 1;
  let max = 0;
  for (const a of aggs) max = Math.max(max, a.count);
  return max || 1;
}

const COUNT_RAMP = ["#e7f0ff", "#1c4ea8"] as const; // light → dark blue
const SEVERITY_RAMP = ["#ffe8cc", "#c92a2a"] as const; // light amber → red
const SENTIMENT = { neg: "#c92a2a", mid: "#ced4da", pos: "#2b8a3e" } as const; // diverging

function lerpChannel(a: number, b: number, t: number): number {
  return Math.round(a + (b - a) * t);
}

function parseHex(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
}

/** Interpolate between two hex colors (t clamped to 0..1). */
export function lerpHex(from: string, to: string, t: number): string {
  const c = Math.max(0, Math.min(1, t));
  const [r1, g1, b1] = parseHex(from);
  const [r2, g2, b2] = parseHex(to);
  const hx = (n: number) => n.toString(16).padStart(2, "0");
  return `#${hx(lerpChannel(r1, r2, c))}${hx(lerpChannel(g1, g2, c))}${hx(lerpChannel(b1, b2, c))}`;
}

/** Fill color for a country's metric value. Sequential for count/severity
 * (floored so a present country is never invisible); diverging for sentiment
 * with a neutral gray midpoint. */
export function fillFor(value: number, metric: Metric, max: number): string {
  if (metric === "sentiment") {
    if (value >= 0) return lerpHex(SENTIMENT.mid, SENTIMENT.pos, value);
    return lerpHex(SENTIMENT.mid, SENTIMENT.neg, -value);
  }
  const t = metric === "count" ? (max > 0 ? value / max : 0) : value; // severity is already a 0..1 share
  const ramp = metric === "severity" ? SEVERITY_RAMP : COUNT_RAMP;
  return lerpHex(ramp[0], ramp[1], Math.max(0.15, Math.min(1, t)));
}

/** Legend endpoints (CSS gradient stops + labels) for the active metric. */
export function legendFor(metric: Metric, max: number): { stops: string[]; min: string; max: string } {
  if (metric === "sentiment") return { stops: [SENTIMENT.neg, SENTIMENT.mid, SENTIMENT.pos], min: "Negative", max: "Positive" };
  if (metric === "severity") return { stops: [...SEVERITY_RAMP], min: "0%", max: "100% high" };
  return { stops: [...COUNT_RAMP], min: "1", max: String(max) };
}
