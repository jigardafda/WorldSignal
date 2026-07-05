// Pure helpers for the live map's marker visuals: severity sizing, recency
// fade, and "breaking" detection. Kept dependency-free so they're trivially
// unit-tested and reused by both the map renderer and the dashboard page.

export interface Recency {
  /** Event id. */
  id: string;
  /** Severity code (LOW | MEDIUM | HIGH | CRITICAL); anything else ⇒ 0 rank. */
  severity?: string | null;
  /** ISO timestamp the event was last seen. */
  lastSeenAt?: string | null;
}

const SEVERITY_RANK: Record<string, number> = { LOW: 0, MEDIUM: 1, HIGH: 2, CRITICAL: 3 };

/** Ordinal severity, 0 (LOW/unknown) … 3 (CRITICAL). */
export function severityRank(severity?: string | null): number {
  return severity ? (SEVERITY_RANK[severity] ?? 0) : 0;
}

/** Marker diameter in px, scaled by severity. LOW=10 … CRITICAL=22. */
export function markerSize(severity?: string | null): number {
  return 10 + severityRank(severity) * 4;
}

/** True for events worth a "breaking" alert: HIGH or CRITICAL severity. */
export function isBreaking(severity?: string | null): boolean {
  return severityRank(severity) >= SEVERITY_RANK.HIGH;
}

/**
 * Marker opacity as a freshness gradient. A just-seen event is fully opaque
 * (1.0); one at the far edge of the window fades to `floor` (0.35). Clamped so
 * out-of-window / future timestamps stay in range. `now`/`windowMs` are passed
 * in so the function is pure and testable.
 */
export function recencyOpacity(lastSeenAt: string | null | undefined, now: number, windowMs: number, floor = 0.35): number {
  if (!lastSeenAt || windowMs <= 0) return 1;
  const t = Date.parse(lastSeenAt);
  if (Number.isNaN(t)) return 1;
  const age = now - t;
  if (age <= 0) return 1;
  if (age >= windowMs) return floor;
  return 1 - (age / windowMs) * (1 - floor);
}

/**
 * Ids of records that are both new this poll (not in `prevIds`) and breaking.
 * `first` suppresses the very first poll so the initial backfill doesn't fire a
 * storm of alerts. Returns records (not just ids) so callers can build toasts.
 */
export function newBreaking<T extends Recency>(records: T[], prevIds: Set<string>, first: boolean): T[] {
  if (first) return [];
  return records.filter((r) => !prevIds.has(r.id) && isBreaking(r.severity));
}
