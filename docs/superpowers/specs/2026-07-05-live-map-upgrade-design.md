# Live Map upgrade — richer markers, instant load, offline

Date: 2026-07-05
Status: Approved

## Goal

Turn the `/live` dashboard from "dots appearing" into a live situation board:
markers that encode severity and freshness, a density-aware view (pins /
clusters / heat), a "breaking" alert for high-severity events, an
instant-loading cache so reopening or reloading paints immediately, and an
offline-ready basemap that reuses tiles already viewed.

## Background / current state

- `/live` → `LiveDashboard` polls `api.liveSignals(since, country, 500)` every
  4s and plots Leaflet pulse markers, color-coded by taxonomy category. It has
  time-window, country, and category/subcategory layer filters; a click opens
  `SignalDrawer`. Filters live in the URL.
- The live feed row carries only: `id, title, country, region, city, severity,
  eventType, lastSeenAt`. `severity` is present but unused visually.
- **The 200 cap:** `backend/internal/db/signals.go` hard-clamps every `signals`
  query to `limit <= 200`, so the map can never show more than 200 events even
  though the frontend asks for 500.
- WebSQL is removed from modern browsers; IndexedDB is the storage primitive.

## Decisions

- **Marker ceiling:** raise backend clamp `200 → 5000` (safety ceiling, not
  unbounded). Frontend fetches up to `2000`/poll; clustering keeps it smooth.
- **Offline scope:** cache *visited* tiles (service-worker runtime cache) +
  signal data in IndexedDB. No full-world tile download.
- **Storage:** IndexedDB, hand-rolled thin wrapper, tested with
  `fake-indexeddb`. Cache is per-browser, cleared on logout.
- **Heat weight:** severity-weighted density.

## Design

### A. Lift the 200 cap
- `signals.go`: change clamp to `5000`. Add a DB test proving > 200 rows return.
- Frontend `MAX_MARKERS` 500 → 2000.

### B. Severity-scaled markers
- Thread `severity` through `MarkerRec → MapMarker`.
- `LiveMap.css`: marker size + glow scale by severity. CRITICAL/HIGH large & hot,
  MEDIUM mid, LOW small & quiet. Category = color; severity = size/intensity.

### C. Recency fade
- Each poll computes `age = now − lastSeenAt` normalized over the window;
  opacity/scale interpolate fresh→bright, old→dim. Pure function
  `recencyOpacity(lastSeenAt, now, windowMs)`.

### D. Cluster / heatmap toggle
- Segmented control **Pins · Clusters · Heat** in the top bar; state in URL
  (`view=`). `LiveMap` gains a `mode` prop.
- Clusters: `leaflet.markercluster`. Heat: `leaflet.heat`, severity-weighted.

### E. "Breaking" burst + toast
- New signal with severity HIGH or CRITICAL (`isNew && isBreaking(severity)`)
  gets a stronger `ws-pulse-breaking` ripple and a throttled/aggregated Mantine
  notification with a "fly to" action. Pure `isBreaking(severity)` +
  `newBreaking(prevIds, recs)` helpers.

### F. Instant-load cache (IndexedDB)
- `lib/signalCache.ts`: `getCached(sinceMs)`, `mergeCached(records)`,
  `clearCache()`. Store keyed by `id`; index on `lastSeenAt`.
- On mount: read cache → render immediately → then poll → merge each poll.
- Eviction: drop rows older than 24h and cap ~5000 (evict oldest).
- `clearCache()` called from `auth.logout()`.

### G. Offline-ready map (service worker)
- `vite-plugin-pwa` (Workbox): precache app shell; runtime-cache OSM tiles
  (StaleWhileRevalidate, capped count + expiration). Offline indicator in the
  top bar via `navigator.onLine` + online/offline events.

## Modules / boundaries

- `lib/liveMarkers.ts` — pure helpers: `severityRank`, `markerSize`,
  `recencyOpacity`, `isBreaking`, `newBreaking`. Fully unit-tested.
- `lib/signalCache.ts` — IndexedDB wrapper. Unit-tested with `fake-indexeddb`.
- `components/LiveMap.tsx` — gains `mode`, per-marker `severity`/`opacity`;
  renders pins/clusters/heat.
- `pages/LiveDashboard.tsx` — cache-first load, view toggle, breaking toasts,
  offline indicator.
- `lib/auth.tsx` — `clearCache()` on logout.

## Testing / verification

- Repo gates ≥95% coverage. Pure helpers + cache get unit tests; Leaflet-plugin
  and SW glue kept thin, logic pushed into tested helpers; Playwright e2e for
  the page.
- Manual end-to-end in Chrome: load `/live`, watch a poll add markers, toggle
  Pins/Clusters/Heat, confirm severity sizing + recency fade, trigger a breaking
  toast, reload → instant cache paint, DevTools offline → tiles + markers still
  render. Capture a GIF.

## New dependencies

`leaflet.markercluster`, `leaflet.heat` (+ `@types`), `vite-plugin-pwa` (dev),
`fake-indexeddb` (dev).

## Out of scope

Full offline world basemap; per-user cache partitioning; server-side entity
time-windowing.
