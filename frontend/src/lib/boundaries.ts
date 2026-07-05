type FeatureMap = Map<string, GeoJSON.Feature>;
let cache: Promise<FeatureMap> | null = null;

// Load + index the simplified world boundaries by ISO alpha-2 code. The dataset,
// the TopoJSON converter, and the ISO code map are all code-split so nothing
// loads until a country is first focused on the live map.
async function loadFeatureMap(): Promise<FeatureMap> {
  if (!cache) {
    cache = (async () => {
      const [topojson, topoMod, isoMod] = await Promise.all([
        import("topojson-client"),
        import("world-atlas/countries-110m.json"),
        import("i18n-iso-countries"),
      ]);
      const iso = isoMod.default;
      const topo = ((topoMod as { default?: unknown }).default ?? topoMod) as Parameters<typeof topojson.feature>[0] & {
        objects: { countries: Parameters<typeof topojson.feature>[1] };
      };
      const fc = topojson.feature(topo, topo.objects.countries) as unknown as GeoJSON.FeatureCollection;
      const map: FeatureMap = new Map();
      for (const f of fc.features) {
        if (f.id == null) continue;
        const alpha2 = iso.numericToAlpha2(String(f.id));
        if (alpha2) map.set(alpha2, f);
      }
      return map;
    })();
  }
  return cache;
}

/** The boundary polygon (GeoJSON Feature) for an ISO alpha-2 country code, or
 * null when the simplified dataset has no geometry for it. */
export async function countryOutline(alpha2: string): Promise<GeoJSON.Feature | null> {
  const map = await loadFeatureMap();
  return map.get(alpha2.toUpperCase()) ?? null;
}

/** All country boundaries indexed by ISO alpha-2 (shared cached map — treat as
 * read-only). Used by the choropleth to paint every country at once. */
export async function allCountryOutlines(): Promise<FeatureMap> {
  return loadFeatureMap();
}

/** Test hook: drop the cached boundary index. */
export function _resetBoundaryCache() {
  cache = null;
}
