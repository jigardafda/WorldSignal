import { useEffect, useState } from "react";
import { api, type Country } from "./api";

// Module-level cache so the country list is fetched at most once per session,
// shared by every CountrySelect / display lookup.
let cache: Promise<Country[]> | null = null;

export function loadCountries(): Promise<Country[]> {
  if (!cache) cache = api.countries().catch((e) => { cache = null; throw e; });
  return cache;
}

// For tests: reset the memoized promise.
export function _resetCountriesCache() { cache = null; }

export interface CountriesState {
  list: Country[];
  byCode: Record<string, Country>;
  loading: boolean;
}

export function useCountries(): CountriesState {
  const [list, setList] = useState<Country[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let active = true;
    loadCountries()
      .then((cs) => { if (active) setList(cs); })
      .catch(() => { if (active) setList([]); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);
  const byCode: Record<string, Country> = {};
  for (const c of list) byCode[c.code] = c;
  return { list, byCode, loading };
}

/** Display label: "United States (🇺🇸)". */
export function countryLabel(c: Country): string {
  return `${c.name} (${c.flag})`;
}

/** Render a country code as "🇺🇸 United States", or the raw code/"—" if unknown. */
export function countryDisplay(code: string | null | undefined, byCode: Record<string, Country>): string {
  if (!code) return "—";
  const c = byCode[code];
  return c ? `${c.flag} ${c.name}` : code;
}
