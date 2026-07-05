// Pure computations for the live side-rails: velocity (events/min), top movers
// (what's surging within the window), and the newest-first ticker. Kept
// dependency-free and time-injected (`now` passed in) so they're trivially
// unit-tested and reused by the panel components.

/** Minimal shape the panels need from a live marker record. */
export interface PulseRec {
  id: string;
  title: string;
  category: string; // taxonomy domain code (POLITICS, DISASTER, …)
  country: string;
  color?: string;
  lat: number;
  lng: number;
  lastSeenMs: number; // epoch ms; 0/NaN treated as "unknown / very old"
}

const MINUTE = 60_000;

/** Events seen in the last 60s = current per-minute rate, overall and per category. */
export function velocity(recs: PulseRec[], now: number): { perMin: number; byCategory: Record<string, number> } {
  const cutoff = now - MINUTE;
  let perMin = 0;
  const byCategory: Record<string, number> = {};
  for (const r of recs) {
    if (r.lastSeenMs >= cutoff) {
      perMin++;
      byCategory[r.category] = (byCategory[r.category] ?? 0) + 1;
    }
  }
  return { perMin, byCategory };
}

export interface Mover {
  key: string;
  recent: number; // count in the recent half of the window
  older: number; // count in the older half
  ratio: number; // recent / older (older 0 ⇒ ratio = recent, i.e. "brand new")
}

/**
 * Split the window `[now-windowMs, now]` at its midpoint and compare each key's
 * count in the recent half vs the older half — surfacing what's *surging now*
 * relative to earlier in the window (no extra fetch needed). `keyOf` selects the
 * dimension (category or country). Only keys active in the recent half are
 * returned, sorted by how hard they're rising, capped at `limit`.
 */
export function topMovers(recs: PulseRec[], keyOf: (r: PulseRec) => string, now: number, windowMs: number, limit = 3): Mover[] {
  const lo = now - windowMs;
  const mid = now - windowMs / 2;
  const recent: Record<string, number> = {};
  const older: Record<string, number> = {};
  for (const r of recs) {
    if (!(r.lastSeenMs >= lo)) continue; // excludes 0/NaN and out-of-window
    const k = keyOf(r);
    if (!k) continue;
    if (r.lastSeenMs >= mid) recent[k] = (recent[k] ?? 0) + 1;
    else older[k] = (older[k] ?? 0) + 1;
  }
  const movers: Mover[] = [];
  for (const k of new Set([...Object.keys(recent), ...Object.keys(older)])) {
    const rc = recent[k] ?? 0;
    if (rc === 0) continue; // only surface things active in the recent half
    const ol = older[k] ?? 0;
    movers.push({ key: k, recent: rc, older: ol, ratio: ol === 0 ? rc : rc / ol });
  }
  movers.sort((a, b) => b.ratio - a.ratio || b.recent - a.recent || a.key.localeCompare(b.key));
  return movers.slice(0, limit);
}

/** Newest-first (by lastSeenMs), capped at `limit`. Does not mutate the input. */
export function tickerItems<T extends { lastSeenMs: number }>(recs: T[], limit = 30): T[] {
  return [...recs].sort((a, b) => b.lastSeenMs - a.lastSeenMs).slice(0, limit);
}

/** Compact relative age ("12s", "3m", "5h", "2d") from epoch ms; "—" if unknown. */
export function timeAgo(ms: number, now: number): string {
  if (!ms || Number.isNaN(ms)) return "—";
  const s = Math.max(0, Math.round((now - ms) / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h`;
  return `${Math.round(h / 24)}d`;
}
