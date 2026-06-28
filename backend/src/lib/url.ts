// Aggressive URL canonicalization so the same article fetched from different
// places (with tracking params, mixed casing, trailing slashes) dedupes cleanly.

const TRACKING_PARAMS = [
  "utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
  "fbclid", "gclid", "mc_cid", "mc_eid", "ref", "ref_src", "igshid",
  "_hsenc", "_hsmi", "spm",
];

export function canonicalizeUrl(input: string | undefined | null): string | null {
  if (!input) return null;
  let raw = input.trim();
  if (!raw) return null;
  try {
    const u = new URL(raw);
    u.hash = "";
    u.hostname = u.hostname.toLowerCase().replace(/^www\./, "");
    // (default ports like :80/:443 are dropped automatically by the URL parser)
    for (const p of TRACKING_PARAMS) u.searchParams.delete(p);
    // sort remaining params for stable ordering
    u.searchParams.sort();
    // strip trailing slash from the path, but keep the root "/"
    if (u.pathname.length > 1 && u.pathname.endsWith("/")) {
      u.pathname = u.pathname.replace(/\/+$/, "");
    }
    return u.toString();
  } catch {
    return raw; // not a valid URL — keep as-is so we don't lose evidence
  }
}
