/** Web-Mercator latitude is only valid to ~±85°; clamp to keep markers on-map. */
export function clampLat(lat: number): number {
  return Math.max(-85, Math.min(85, lat));
}

/** Deterministically scatter a coordinate by a small amount derived from a seed
 * (the signal id). Events that share a country's capital coordinates then spread
 * into a readable cluster instead of stacking on one pixel. Stable across renders. */
export function jitter(lat: number, lng: number, seed: string): [number, number] {
  let h = 2166136261;
  for (let i = 0; i < seed.length; i++) {
    h = (h ^ seed.charCodeAt(i)) >>> 0;
    h = (h * 16777619) >>> 0;
  }
  const dx = ((h % 1000) / 1000 - 0.5) * 1.4; // ±0.7°
  const dy = (((h >>> 10) % 1000) / 1000 - 0.5) * 1.4;
  return [clampLat(lat + dy), lng + dx];
}
