import { useEffect, useRef, useState } from "react";
import { Badge, Group, Text } from "@mantine/core";
import { IconBroadcast } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useCountries } from "../lib/countries";
import { CountrySelect } from "../components/CountrySelect";
import { LiveMap, type MapMarker } from "../components/LiveMap";
import { jitter } from "../lib/geo";

const POLL_MS = 4000;
const MAX_MARKERS = 500;
const WORLD_CENTER: [number, number] = [20, 0];

type MarkerRec = MapMarker & { country: string };

/** Live Mode: a continuously-updating world map. Polls the signal feed, places a
 * glowing pulse marker at each event's country location, and reframes to the
 * whole world or a single country. No page refresh required. */
export function LiveDashboard() {
  const { byCode, list } = useCountries();
  const [country, setCountry] = useState<string | null>(null);
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
        store.set(s.id, { id: s.id, lat, lng, title: s.title, country: s.country });
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
  const shown = country ? markers.filter((m) => m.country === country) : markers;

  return (
    <div style={{ height: "calc(100dvh - 56px)", display: "flex", flexDirection: "column" }} data-testid="live-dashboard">
      <Group justify="space-between" px="md" py="xs" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
        <Group gap="xs">
          <Badge color="blue" variant="light" leftSection={<IconBroadcast size={12} />} data-testid="live-indicator">Live</Badge>
          <Text size="sm" c="dimmed">{shown.length} events on map{lastUpdate && ` · updated ${lastUpdate}`}</Text>
        </Group>
        <CountrySelect placeholder="Whole world" value={country} onChange={setCountry} data-testid="live-country" />
      </Group>
      <div style={{ flex: 1, minHeight: 0 }}>
        <LiveMap markers={shown} center={center} zoom={zoom} height="100%" />
      </div>
    </div>
  );
}
