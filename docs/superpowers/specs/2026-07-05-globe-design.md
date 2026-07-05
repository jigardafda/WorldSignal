# Live map — 3D Globe view mode

Date: 2026-07-05
Status: Approved

## Goal

Add a 3D globe as a first-class live-map view with **parity** to the 2D map:
every data/interaction/overlay feature carries over. Built in two phases; the
whole feature must ship.

## Library & isolation

- `react-globe.gl` (three.js). **Lazy-loaded** (React.lazy + Suspense) so three
  loads only when the Globe view is opened; split into its own `globe` vite
  chunk and kept out of the SW precache (runtime-cached like `maps`). WebGL can't
  run in jsdom, so 3D rendering is browser-verified; logic lives in tested pure
  helpers and the component is unit-tested with `react-globe.gl` mocked.
- **Base globe = country polygons** from `allCountryOutlines()` (world-atlas we
  already load) — no external earth texture (CSP-safe, offline-friendly).

## Parity mapping (2D → globe)

| 2D feature | Globe |
|---|---|
| Category color | point color |
| Severity → size | point radius/altitude |
| Recency fade | point opacity |
| Breaking ripple (new HIGH/CRIT) | pulsing ring (`ringsData`) |
| Breaking toast | global overlay (already works) |
| Click → drawer | `onPointClick` → `onSelect` |
| Ticker fly-to | `pointOfView` rotate to marker |
| Country focus | highlight polygon + rotate to it |
| Sentiment tint | points colored by sentiment when on |
| Corroboration ring (sourceCount) | point ring/size |
| Influence / window / category / country filters | shape `shown` → points (automatic) |
| Timeline replay | feed `displayMarkers` (frameMarkers) → points sweep |
| Choropleth metrics (Regions) | color globe polygons by metric (Phase 2) |
| Arcs (new) | chronological activity thread |

Intentionally **omitted** (2D-density tricks with no 3D-native meaning):
clustering and heat.

## Phases

### Phase 1 — Core globe (PR 1)
- `lib/globeData.ts` (pure, tested): `toPoints(markers, sentimentTint)`,
  `toArcs(markers, maxArcs)`, ring/size/color helpers.
- `components/LiveGlobe.tsx`: wraps `react-globe.gl`; loads polygons; feeds
  points/arcs/rings; auto-rotate (pauses on interaction); `focus`/`flyTo` via
  `pointOfView`; `onSelect`; `sentimentTint`. Sized via a container ref.
- `pages/LiveDashboard.tsx`: add `Globe` to the view control; when `view=globe`
  render the lazy globe (Suspense) in place of `LiveMap`, fed `displayMarkers`
  (so replay works), `sentimentTint`, `focus`, `flyTo`, `onSelect`.
- `vite.config.ts`: `globe` chunk; SW globIgnore + runtime-cache.

### Phase 2 — Globe choropleth (PR 2)
- In Globe view, honor the metric select + legend: color the globe's country
  polygons by count/severity/sentiment (reuse `lib/choropleth.ts`). Points can
  hide or dim in this sub-mode. Legend shown.

## Testing / verification (each phase)
- Pure helpers fully unit-tested; `LiveGlobe` tested with `react-globe.gl`
  mocked (asserts pointsData/arcsData/polygonsData + click wiring); LiveDashboard
  integration (switch to Globe mounts globe, unmounts 2D map).
- Frontend coverage ≥95%. **No backend changes.**
- Browser E2E: rotate, points, arcs, click→drawer, fly-to, replay (Phase 1);
  metric coloring (Phase 2). Screenshots/GIF to mobile.
- CI green → merge each PR to main before starting the next.

## Out of scope
Clustering / heat on the globe; external earth textures; server-side data.
