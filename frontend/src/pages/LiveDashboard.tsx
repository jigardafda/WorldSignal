import { useEffect, useRef, useState } from "react";
import { Badge, Group, Text, UnstyledButton } from "@mantine/core";
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

type MarkerRec = MapMarker & { country: string; category: string };

/** Live Mode: a continuously-updating world map. Polls the signal feed, places a
 * pulse marker at each event's country location color-coded by taxonomy category,
 * and supports filtering by country and by category layer. No page refresh. */
export function LiveDashboard() {
  const { byCode, list } = useCountries();
  const [country, setCountry] = useState<string | null>(null);
  const [enabled, setEnabled] = useState<string[]>(CATEGORIES.map((c) => c.code));
  const [markers, setMarkers] = useState<MarkerRec[]>([]);
  const [lastUpdate, setLastUpdate] = useState<string>("");
  const storeRef = useRef<Map<string, MarkerRec>>(new Map());

  useEffect(() => {
    if (list.length === 0) return; // wait until country coordinates are loaded
    const coords = new Map(list.map((c) => [c.code, c]));
    let active = true;
    async function poll() {
      let signals;
      try {
        signals = await api.liveSignals(150);
      } catch {
        return; // transient; keep polling
      }
      if (!active) return;
      const store = storeRef.current;
      const fresh = new Set<string>();
      for (const s of signals) {
        if (!s.country || store.has(s.id)) continue;
        const c = coords.get(s.country);
        if (!c || (!c.capitalLat && !c.capitalLng)) continue;
        const [lat, lng] = jitter(c.capitalLat, c.capitalLng, s.id);
        const category = domainOf(s.eventType);
        store.set(s.id, { id: s.id, lat, lng, title: s.title, color: categoryColor(category), country: s.country, category });
        fresh.add(s.id);
      }
      while (store.size > MAX_MARKERS) {
        const oldest = store.keys().next().value as string;
        store.delete(oldest);
      }
      setMarkers([...store.values()].map((m) => ({ ...m, isNew: fresh.has(m.id) })));
      if (fresh.size) setLastUpdate(new Date().toLocaleTimeString());
    }
    void poll();
    const t = setInterval(() => void poll(), POLL_MS);
    return () => {
      active = false;
      clearInterval(t);
    };
  }, [list]);

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
        <CountrySelect placeholder="Whole world" value={country} onChange={setCountry} data-testid="live-country" />
      </Group>
      <Group gap={6} px="md" py="xs" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }} data-testid="live-legend">
        {CATEGORIES.map((c) => {
          const on = enabled.includes(c.code);
          return (
            <UnstyledButton key={c.code} onClick={() => toggle(c.code)} data-testid={`layer-${c.code}`} aria-pressed={on}>
              <Badge
                variant={on ? "filled" : "outline"}
                color={c.color}
                style={{ opacity: on ? 1 : 0.45, cursor: "pointer" }}
              >
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
