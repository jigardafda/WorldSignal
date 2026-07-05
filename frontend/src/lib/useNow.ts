import { useEffect, useState } from "react";

/** A clock that re-renders the calling component every `intervalMs` (default 1s),
 * so relative times / rolling counts stay fresh without touching the map. */
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(t);
  }, [intervalMs]);
  return now;
}
