import { useEffect, useRef, useState } from "react";
import { Badge, Group, Select, Text, UnstyledButton } from "@mantine/core";
import { IconBroadcast } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useCountries } from "../lib/countries";
import { CountrySelect } from "../components/CountrySelect";
import { LiveMap, type MapMarker } from "../components/LiveMap";
import { jitter } from "../lib/geo";
import { CATEGORIES, categoryColor, categoryLabel, domainOf } from "../lib/categories";

const POLL_MS = 4000;
const MAX_MARKERS = 500;
const WORLD_CENTER: [number, number] = [20, 0];

// Time windows merge past events with live ones; markers age out as time passes.
const WINDOWS = [
  { value: "30", label: "Last 30 min" },
  { value: "60", label: "Last 1 hour" },
  { value: "360", label: "Last 6 hours" },
  { value: "1440", label: "Last 24 hours" },
];

type MarkerRec = MapMarker & { country: string; category: string };

/** Live Mode: a continuously-updating world map. Within the chosen time window it
 * shows past events plus newly-ingested ones, placed at each event's country
 * location and color-coded by taxonomy category. Filter by country, category
 * layer, and time window. No page refresh. */
export function LiveDashboard() {
  const { byCode, list } = useCountries();
  const [country, setCountry] = useState<string | null>(null);
  const [windowMin, setWindowMin] = useState<string>("60");
  const [enabled, setEnabled] = useState<string[]>(CATEGORIES.map((c) => c.code));
  const [markers, setMarkers] = useState<MarkerRec[]>([]);
  const [lastUpdate, setLastUpdate] = useState<string>("");
  const prevIdsRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    if (list.length === 0) return; // wait until country coordinates are loaded
    const coords = new Map(list.map((c) => [c.code, c]));
    const windowMs = Number(windowMin) * 60_000;
    let active = true;
    let first = true;
    async function poll() {
      const since = new Date(Date.now() - windowMs).toISOString();
      let signals;
      try {
        signals = await api.liveSignals(since, MAX_MARKERS);
      } catch {
        return; // transient; keep polling
      }
      if (!active) return;
      const prev = prevIdsRef.current;
      const recs: MarkerRec[] = [];
      const ids = new Set<string>();
      for (const s of signals) {
        if (!s.country) continue;
        const c = coords.get(s.country);
        if (!c || (!c.capitalLat && !c.capitalLng)) continue;
        const [lat, lng] = jitter(c.capitalLat, c.capitalLng, s.id);
        const category = domainOf(s.eventType);
        recs.push({ id: s.id, lat, lng, title: s.title, color: categoryColor(category), country: s.country, category, isNew: !first && !prev.has(s.id) });
        ids.add(s.id);
      }
      const freshly = recs.filter((r) => r.isNew).length;
      prevIdsRef.current = ids;
      first = false;
      setMarkers(recs);
      if (freshly) setLastUpdate(new Date().toLocaleTimeString());
    }
    void poll();
    const t = setInterval(() => void poll(), POLL_MS);
    return () => {
      active = false;
      clearInterval(t);
    };
  }, [list, windowMin]);

  const sel = country ? byCode[country] : null;
  const center: [number, number] = sel && (sel.capitalLat || sel.capitalLng) ? [sel.capitalLat, sel.capitalLng] : WORLD_CENTER;
  const zoom = sel ? 5 : 2;

  const inCountry = country ? markers.filter((m) => m.country === country) : markers;
  const counts: Record<string, number> = {};
  for (const m of inCountry) counts[m.category] = (counts[m.category] ?? 0) + 1;
  const shown = inCountry.filter((m) => enabled.includes(m.category));

  const toggle = (code: string) =>
    setEnabled((prev) => (prev.includes(code) ? prev.filter((x) => x !== code) : [...prev, code]));

  return (
    <div style={{ height: "calc(100dvh - 56px)", display: "flex", flexDirection: "column" }} data-testid="live-dashboard">
      <Group justify="space-between" px="md" py="xs" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
        <Group gap="xs">
          <Badge color="blue" variant="light" leftSection={<IconBroadcast size={12} />} data-testid="live-indicator">Live</Badge>
          <Text size="sm" c="dimmed">{shown.length} events on map{lastUpdate && ` · updated ${lastUpdate}`}</Text>
        </Group>
        <Group gap="xs">
          <Select data={WINDOWS} value={windowMin} onChange={(v) => setWindowMin(v ?? "60")} allowDeselect={false} w={150} data-testid="live-window" />
          <CountrySelect placeholder="Whole world" value={country} onChange={setCountry} data-testid="live-country" />
        </Group>
      </Group>
      <Group gap={6} px="md" py="xs" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }} data-testid="live-legend">
        {CATEGORIES.map((c) => {
          const on = enabled.includes(c.code);
          return (
            <UnstyledButton key={c.code} onClick={() => toggle(c.code)} data-testid={`layer-${c.code}`} aria-pressed={on}>
              <Badge variant={on ? "filled" : "outline"} color={c.color} style={{ opacity: on ? 1 : 0.45, cursor: "pointer" }}>
                {categoryLabel(c.code)} {counts[c.code] ?? 0}
              </Badge>
            </UnstyledButton>
          );
        })}
      </Group>
      <div style={{ flex: 1, minHeight: 0 }}>
        <LiveMap markers={shown} center={center} zoom={zoom} height="100%" />
      </div>
    </div>
  );
}
