// Timeline replay: given a frozen set of markers and a playhead time, produce
// the markers visible at that instant — events up to the playhead, faded by
// their age relative to it, rippling the ones that just crossed it. Pure and
// dependency-free (aside from the shared fade math) so it's trivially tested.

import { recencyOpacityMs } from "./liveMarkers";

/** The fields replay needs; carries the rest of the marker through untouched. */
export interface FrameMarker {
  lastSeenMs: number;
  opacity?: number;
  isNew?: boolean;
}

/**
 * Markers visible at playhead `t`:
 * - events with a real timestamp (`lastSeenMs > 0`) that have occurred by `t`,
 * - opacity faded by age relative to `t` (reuses the live fade),
 * - `isNew` for events that crossed the playhead since `prevPlayheadMs` (so they
 *   ripple as the sweep reaches them); `null` prev (a seek/first frame) ⇒ no ripples.
 * Events without a timestamp can't be placed in time and are omitted.
 */
export function frameMarkers<T extends FrameMarker>(recs: T[], playheadMs: number, windowMs: number, prevPlayheadMs: number | null): T[] {
  const out: T[] = [];
  for (const r of recs) {
    if (!(r.lastSeenMs > 0)) continue; // no timestamp → not placeable
    if (r.lastSeenMs > playheadMs) continue; // hasn't happened yet at t
    const isNew = prevPlayheadMs != null && r.lastSeenMs > prevPlayheadMs;
    out.push({ ...r, opacity: recencyOpacityMs(r.lastSeenMs, playheadMs, windowMs), isNew });
  }
  return out;
}
