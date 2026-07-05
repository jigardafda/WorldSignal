# Live map — Choropleth ("Regions") view mode

Date: 2026-07-05
Status: Approved

## Goal

Add a country-choropleth view to the live map: instead of pins, fill each
country by a chosen metric (count / severity / sentiment) computed from the
currently-visible signals. Pure frontend — the world boundaries are already
loaded and metrics aggregate client-side.

## Design

### A 4th view mode
- View control becomes `Pins · Clusters · Heat · Regions` (URL `view=regions`).
- In Regions mode markers are hidden; every country polygon is filled by the
  metric. Aggregation is over `shown` (the already country/category/influence/
  window-filtered set), so all filters still apply.

### Metric selector (Regions mode only)
- `Count · Severity · Sentiment` (URL `metric=`, default `count`).
  - **Count** — sequential scale by number of signals.
  - **Severity** — sequential by share of HIGH/CRITICAL.
  - **Sentiment** — diverging red↔green by net sentiment `(POS − NEG)/total`.
- Countries with no signals render faint/neutral.
- A color **legend** (scale + endpoints) shows bottom-left.

### Interaction
- Hover a country → tooltip with its count + metric value.
- Click a country → sets it as the country filter (drills in + zooms),
  consistent with the rest of the app.

## Modules (all frontend)
- `lib/boundaries.ts` — add `allCountryOutlines(): Promise<Map<string, Feature>>`
  returning the cached feature map (reuses the existing loader).
- `lib/choropleth.ts` (pure, unit-tested):
  - `aggregateByCountry(markers)` → `Map<alpha2, {count, hiShare, sentimentNet}>`.
  - `metricValue(agg, metric)` and `metricDomain(aggs, metric)`.
  - `fillFor(value, metric, domain)` → hex (sequential for count/severity,
    diverging for sentiment). Palette from the dataviz skill.
- `components/LiveMap.tsx` — `mode="regions"` renders an `L.geoJSON` layer of all
  countries, styled per-country via a passed `regionStyle(alpha2)` fn; hover
  (tooltip) + click (onSelect country) handlers. Markers layer suppressed.
- `pages/LiveDashboard.tsx` — metric Select (Regions only), compute aggregation
  from `shown`, pass style fn + legend; click → setCountry.
- `components/ChoroplethLegend.tsx` — the color scale legend.

## Testing / verification
- Unit: `aggregateByCountry`, `metricValue`, `metricDomain`, `fillFor`,
  `allCountryOutlines`.
- Component: LiveMap regions mode builds a geoJSON layer and hides markers.
- Integration: switching to Regions shows the metric select + legend and hides
  pins; changing metric restyles; clicking a country sets the filter.
- Browser E2E across all three metrics with seeded data + screenshots.
- Frontend coverage ≥95%. **No backend changes.**

## Out of scope
Server-side country aggregation; per-subcategory choropleth; animated
transitions; globe mode (separate follow-up).
