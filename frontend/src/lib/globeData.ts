// Pure transforms from live markers to react-globe.gl layer data: points,
// the chronological "activity thread" arcs, and breaking rings. Dependency-free
// (aside from the shared marker helpers) and unit-tested; the WebGL component
// consumes these so the untestable render surface stays logic-free.

import { sentimentColor, severityRank } from "./liveMarkers";

export interface PointInput {
  id: string;
  lat: number;
  lng: number;
  title: string;
  color?: string; // category color
  severity?: string | null;
  opacity?: number; // recency fade
  sentiment?: string | null;
  isNew?: boolean;
  breaking?: boolean;
  lastSeenMs?: number;
}

export interface GlobePoint { id: string; lat: number; lng: number; title: string; color: string; size: number; opacity: number }
export interface GlobeArc { startLat: number; startLng: number; endLat: number; endLng: number; color: string }
export interface GlobeRing { lat: number; lng: number; color: string }

const DEFAULT = "#2f6df6";

/** Point radius on the globe, scaled by severity (0.18 … 0.48). */
export function pointSize(severity?: string | null): number {
  return 0.18 + severityRank(severity) * 0.1;
}

/** Markers → globe points. When `sentimentTint` is on, color by sentiment
 * instead of category (parity with the 2D tint layer). */
export function toPoints(markers: PointInput[], sentimentTint = false): GlobePoint[] {
  return markers.map((m) => ({
    id: m.id,
    lat: m.lat,
    lng: m.lng,
    title: m.title,
    color: sentimentTint ? sentimentColor(m.sentiment) : m.color ?? DEFAULT,
    size: pointSize(m.severity),
    opacity: m.opacity ?? 1,
  }));
}

/** The chronological activity thread: the most recent `maxArcs` events (by
 * lastSeenMs) connected consecutively newest→older. Events without a timestamp
 * are excluded; fewer than two events ⇒ no arcs. */
export function toArcs(markers: PointInput[], maxArcs = 20): GlobeArc[] {
  const recent = markers
    .filter((m) => (m.lastSeenMs ?? 0) > 0)
    .sort((a, b) => (b.lastSeenMs ?? 0) - (a.lastSeenMs ?? 0))
    .slice(0, maxArcs);
  const arcs: GlobeArc[] = [];
  for (let i = 0; i < recent.length - 1; i++) {
    arcs.push({
      startLat: recent[i].lat,
      startLng: recent[i].lng,
      endLat: recent[i + 1].lat,
      endLng: recent[i + 1].lng,
      color: recent[i].color ?? DEFAULT,
    });
  }
  return arcs;
}

/** Pulsing rings for events that just arrived and are breaking (parity with the
 * 2D breaking ripple). */
export function toRings(markers: PointInput[]): GlobeRing[] {
  return markers.filter((m) => m.breaking && m.isNew).map((m) => ({ lat: m.lat, lng: m.lng, color: m.color ?? "#e03131" }));
}
