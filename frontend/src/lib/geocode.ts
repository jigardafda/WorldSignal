type Coord = { lat: number; lng: number };
export type GeoHit = Coord & { precision: "city" | "region" };

// country-state-city is a large offline DB (code-split). We load it in the
// background; geocode() is synchronous and returns null until it's ready, so the
// map can place capital-fallback markers immediately and upgrade them once loaded.
type CSC = typeof import("country-state-city");
let cscPromise: Promise<CSC> | null = null;
let csc: CSC | null = null;

/** Begin loading the geocoding DB (idempotent). Resolves when ready. */
export function preloadGeo(): Promise<CSC> {
  if (!cscPromise) cscPromise = import("country-state-city").then((m) => (csc = m));
  return cscPromise;
}

const norm = (s: string) => s.trim().toLowerCase();
const stateCache = new Map<string, Map<string, Coord & { iso: string }>>();
const cityCache = new Map<string, Map<string, Coord>>();
type CityLike = { name: string; latitude?: string | null; longitude?: string | null };

function statesOf(country: string): Map<string, Coord & { iso: string }> {
  let m = stateCache.get(country);
  if (!m) {
    m = new Map();
    for (const s of csc!.State.getStatesOfCountry(country)) {
      const lat = Number(s.latitude);
      const lng = Number(s.longitude);
      if (lat || lng) m.set(norm(s.name), { lat, lng, iso: s.isoCode });
    }
    stateCache.set(country, m);
  }
  return m;
}

function citiesOf(key: string, get: () => CityLike[] | undefined): Map<string, Coord> {
  let m = cityCache.get(key);
  if (!m) {
    m = new Map();
    for (const c of get() ?? []) {
      const lat = Number(c.latitude ?? 0);
      const lng = Number(c.longitude ?? 0);
      const k = norm(c.name);
      if ((lat || lng) && !m.has(k)) m.set(k, { lat, lng });
    }
    cityCache.set(key, m);
  }
  return m;
}

/** Most precise coordinates for an extracted place: city → state → null (caller
 * falls back to the country capital). Synchronous and non-blocking: returns null
 * until the offline DB has finished loading. */
export function geocode(country: string | null | undefined, region?: string | null, city?: string | null): GeoHit | null {
  if (!country) return null;
  if (!csc) {
    void preloadGeo();
    return null;
  }
  const c = country.toUpperCase();
  const states = statesOf(c);
  const stateHit = region ? states.get(norm(region)) : undefined;

  if (city) {
    const key = norm(city);
    if (stateHit) {
      const within = citiesOf(`${c}:${stateHit.iso}`, () => csc!.City.getCitiesOfState(c, stateHit.iso));
      const hit = within.get(key);
      if (hit) return { ...hit, precision: "city" };
    }
    const all = citiesOf(c, () => csc!.City.getCitiesOfCountry(c));
    const hit = all.get(key);
    if (hit) return { ...hit, precision: "city" };
  }
  if (stateHit) return { lat: stateHit.lat, lng: stateHit.lng, precision: "region" };
  return null;
}

/** Test hook: clear cached indexes (does not unload the DB). */
export function _resetGeoCache() {
  stateCache.clear();
  cityCache.clear();
}
