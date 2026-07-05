/** Format an ISO timestamp as a locale string, or a dash when empty. */
export function fmtDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString();
}

function ordinal(n: number): string {
  const s = ["th", "st", "nd", "rd"];
  const v = n % 100;
  return s[(v - 20) % 10] || s[v] || s[0];
}

/** Format an ISO timestamp as a friendly day, e.g. "5th July 2026" (no time). */
export function fmtDay(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  const day = d.getDate();
  return `${day}${ordinal(day)} ${d.toLocaleString(undefined, { month: "long" })} ${d.getFullYear()}`;
}

/** Format a 0..1 confidence/credibility as a percentage. */
export function pct(value: number | null | undefined): string {
  if (value == null) return "—";
  return `${Math.round(value * 100)}%`;
}

const LANGUAGE_NAMES: Record<string, string> = {
  en: "English", fr: "French", es: "Spanish", de: "German", it: "Italian",
  pt: "Portuguese", ru: "Russian", zh: "Chinese", ja: "Japanese", ko: "Korean",
  ar: "Arabic", hi: "Hindi", bn: "Bengali", ur: "Urdu", fa: "Persian",
  tr: "Turkish", nl: "Dutch", pl: "Polish", uk: "Ukrainian", vi: "Vietnamese",
  th: "Thai", id: "Indonesian", he: "Hebrew", sv: "Swedish", el: "Greek",
};

/** Human-readable language name for an ISO 639-1 code, falling back to the code. */
export function languageName(code: string | null | undefined): string {
  if (!code) return "";
  return LANGUAGE_NAMES[code.toLowerCase()] ?? code.toUpperCase();
}

/** Relative "x ago" string for an ISO timestamp. */
export function timeAgo(iso: string | null | undefined): string {
  if (!iso) return "never";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "never";
  const secs = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.round(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.round(hrs / 24)}d ago`;
}
