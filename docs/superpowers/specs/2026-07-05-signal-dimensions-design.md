# Live map — deeper signal dimensions (sentiment, corroboration, influence)

Date: 2026-07-05
Status: Approved

## Goal

Surface three enrichment dimensions already produced by the pipeline on the live
map: **sentiment** (tint), **source corroboration** (ring), and **influence**
(filter) — with no backend changes.

## Research outcome (why this is frontend-only)

- The `signals` resolver already returns `sentiment, sentimentScore, influence,
  relevance, sourceCount` (`graphql_route.go:123–129`); `signalScalarCols`
  already selects them and the GraphQL executor is schema-less. The `liveSignals`
  query just isn't requesting them.
- `SignalFilter` already supports `sentiment`/`influence`/`minRelevance`
  server-side, but we filter **client-side** to match the category-layer pattern
  and get ≥-threshold semantics.
- Value domains: `sentiment` = POSITIVE/NEGATIVE/NEUTRAL (MIXED from LLM);
  `influence` = LOW/MEDIUM/HIGH; `relevance`/`sentimentScore` = floats;
  `sourceCount` = int ≥1.
- Population caveat (keyless/heuristic): `sentiment`, `sentimentScore`,
  `relevance` are always set; **`influence` is usually null**; `sourceCount` is
  ≥1 but only >1 after clustering. The UI must handle nulls; demo data is seeded
  with varied values.

## Design (all frontend)

### Shared plumbing
- Add `sentiment, influence, relevance, sourceCount` to `LiveSignal` and to the
  `liveSignals` query selection (`api.ts`).
- Thread `sourceCount, sentiment, influence` through `buildRecs` → `MarkerRec`
  (`LiveDashboard.tsx`).
- New pure helpers in `lib/liveMarkers.ts` (tested, mirroring `severityRank`):
  - `ringWidth(sourceCount)` → px (1 → 0, scaling, capped ~6).
  - `influenceRank(influence)` → 0..3 (null/unknown → 0).
  - `sentimentColor(sentiment)` → hex (reusing the badge palette).

### 1. Corroboration ring (always-on)
- A `::after` ring on `.ws-pulse`, thickness driven by a new `--ws-ring` custom
  property = `ringWidth(sourceCount)`. Single-source ⇒ no ring; more sources ⇒
  thicker outline. No control.

### 2. Sentiment tint (opt-in layer)
- Top-bar toggle → URL param `sent=1` (default omitted). When on, `buildMarker`
  adds a `ws-tint` class and a `--ws-a` accent = `sentimentColor(sentiment)`,
  coloring the marker's border while keeping the category fill. `LiveMap` gains a
  `sentimentTint` prop; markers carry `sentiment`.

### 3. Influence filter (top-bar Select)
- All / Medium+ / High → URL param `infl` (default omitted). Extends the `shown`
  filter with `influenceRank(m.influence) >= threshold`; null influence is
  excluded above "All".

### Rendering hooks
- `LiveMap.tsx buildMarker`: emit `--ws-ring` (if >0) and, when tinting,
  `ws-tint` + `--ws-a`. All values regex-safe (`COLOR_RE`).
- `LiveMap.css`: `.ws-pulse { position: relative }`, `.ws-pulse::after` ring,
  `.ws-pulse.ws-tint { border-color: var(--ws-a) }`.

## Testing / verification
- Unit: `ringWidth`, `influenceRank`, `sentimentColor` in `liveMarkers.test.ts`;
  marker-HTML assertions (`--ws-ring`, `ws-tint`, `--ws-a`) in `LiveMap.test.tsx`.
- Integration: influence filter reduces the shown set; sentiment toggle passes
  `sentimentTint` to the map (`LiveDashboard.test.tsx`).
- Frontend coverage ≥95%. **No backend changes** (backend stays green).
- Browser E2E with seeded varied sentiment/influence/sourceCount + screenshots.

## Out of scope

Relevance slider/filter; a separate sentiment filter; server-side filtering;
`sentimentScore` gradient tint.
