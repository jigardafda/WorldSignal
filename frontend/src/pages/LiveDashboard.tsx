import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ActionIcon, Anchor, Badge, Checkbox, ColorSwatch, Group, Paper, SegmentedControl, Select, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconAlertTriangle, IconBroadcast, IconChevronDown, IconChevronUp, IconStack2, IconWifiOff } from "@tabler/icons-react";
import { api, type LiveSignal, type TaxonomyNode } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useCountries } from "../lib/countries";
import { CountrySelect } from "../components/CountrySelect";
import { LiveMap, type MapMarker, type MapMode } from "../components/LiveMap";
import { SignalDrawer } from "../components/SignalDrawer";
import { jitter } from "../lib/geo";
import { geocode, preloadGeo } from "../lib/geocode";
import { CATEGORIES, categoryColor, categoryLabel, domainOf } from "../lib/categories";
import { isBreaking, newBreaking, recencyOpacity } from "../lib/liveMarkers";
import { getCached, mergeCached } from "../lib/signalCache";

const POLL_MS = 4000;
const MAX_MARKERS = 2000;
const WORLD_CENTER: [number, number] = [20, 0];

const VIEWS: { value: MapMode; label: string }[] = [
  { value: "pins", label: "Pins" },
  { value: "cluster", label: "Clusters" },
  { value: "heat", label: "Heat" },
];
const isMode = (v: string | null): v is MapMode => v === "cluster" || v === "heat";

/** Toast for newly-arrived breaking (HIGH/CRITICAL) signals, aggregated so a
 * burst is one notification. The first one is clickable to open its drawer. */
function notifyBreaking(brk: LiveSignal[], onOpen: (id: string) => void) {
  const first = brk[0];
  notifications.show({
    color: "red",
    icon: <IconAlertTriangle size={16} />,
    title: brk.length === 1 ? "Breaking signal" : `${brk.length} breaking signals`,
    message: (
      <Anchor size="sm" onClick={() => onOpen(first.id)}>
        {brk.length === 1 ? first.title : brk.slice(0, 3).map((b) => b.title).join(" · ")}
      </Anchor>
    ),
    autoClose: 8000,
  });
}

// Time windows merge past events with live ones; markers age out as time passes.
const WINDOWS = [
  { value: "30", label: "Last 30 min" },
  { value: "60", label: "Last 1 hour" },
  { value: "360", label: "Last 6 hours" },
  { value: "1440", label: "Last 24 hours" },
];

type MarkerRec = MapMarker & { country: string; category: string; leaf: string };

function setOrDelete(p: URLSearchParams, key: string, value: string) {
  if (value) p.set(key, value);
  else p.delete(key);
}

/** Live Mode: a continuously-updating world map. Within the chosen time window it
 * shows past events plus newly-ingested ones, placed at each event's country
 * location and color-coded by taxonomy category. Layers can be filtered at the
 * category level or, by expanding a category, at the subcategory level. Filter
 * by country and time window too. State lives in the URL; no page refresh. */
export function LiveDashboard() {
  const { byCode, list } = useCountries();
  const tax = useAsync<TaxonomyNode[]>(() => api.taxonomy(), []);
  const leavesByDomain = new Map<string, TaxonomyNode[]>();
  for (const d of tax.data ?? []) leavesByDomain.set(d.code, d.children ?? []);

  // Filters live in the URL so they're retained on reload and shareable.
  const [params, setParams] = useSearchParams();
  const windowMin = params.get("w") ?? "60";
  const country = params.get("country");
  const disabledDomains = new Set((params.get("off") ?? "").split(",").filter(Boolean));
  const disabledLeaves = new Set((params.get("offsub") ?? "").split(",").filter(Boolean));

  const update = (mut: (p: URLSearchParams) => void) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        mut(next);
        return next;
      },
      { replace: true },
    );
  const setOne = (key: string, value: string) => update((p) => setOrDelete(p, key, value));
  const setCountry = (v: string | null) => setOne("country", v ?? "");
  const setWindowMin = (v: string) => setOne("w", v === "60" ? "" : v); // 60 = default → omit
  const view: MapMode = isMode(params.get("view")) ? (params.get("view") as MapMode) : "pins";
  const setView = (v: string) => setOne("view", v === "pins" ? "" : v); // pins = default → omit

  const toggleDomain = (code: string) =>
    update((p) => {
      const dom = new Set((p.get("off") ?? "").split(",").filter(Boolean));
      const leaves = new Set((p.get("offsub") ?? "").split(",").filter(Boolean));
      if (dom.has(code)) {
        dom.delete(code); // re-enabling a category clears any per-subcategory exclusions
        for (const l of leavesByDomain.get(code) ?? []) leaves.delete(l.code);
      } else {
        dom.add(code);
      }
      setOrDelete(p, "off", [...dom].join(","));
      setOrDelete(p, "offsub", [...leaves].join(","));
    });
  const toggleLeaf = (code: string) =>
    update((p) => {
      const leaves = new Set((p.get("offsub") ?? "").split(",").filter(Boolean));
      if (leaves.has(code)) leaves.delete(code);
      else leaves.add(code);
      setOrDelete(p, "offsub", [...leaves].join(","));
    });

  const [markers, setMarkers] = useState<MarkerRec[]>([]);
  const [lastUpdate, setLastUpdate] = useState<string>("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [legendOpen, setLegendOpen] = useState(true);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const toggleExpand = (code: string) =>
    setExpanded((prev) => {
      const n = new Set(prev);
      if (n.has(code)) n.delete(code);
      else n.add(code);
      return n;
    });
  const prevIdsRef = useRef<Set<string>>(new Set());
  const coordRef = useRef<Map<string, [number, number]>>(new Map());
  const [online, setOnline] = useState(() => (typeof navigator === "undefined" ? true : navigator.onLine));

  // Track connectivity so the header can show an offline indicator (the map
  // still renders from the tile cache + IndexedDB while offline).
  useEffect(() => {
    const on = () => setOnline(true);
    const off = () => setOnline(false);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
    };
  }, []);

  useEffect(() => {
    if (list.length === 0) return; // wait until country coordinates are loaded
    void preloadGeo(); // warm the precise-geocoding DB in the background
    const coords = new Map(list.map((c) => [c.code, c]));
    const windowMs = Number(windowMin) * 60_000;
    let active = true;
    let first = true;

    // Turn a batch of live signals into map markers: geolocate, color by
    // category, size by severity, and fade by recency. `flagNew` marks events
    // unseen last poll so they ripple (suppressed on the very first paint).
    function buildRecs(signals: LiveSignal[], flagNew: boolean): { recs: MarkerRec[]; ids: Set<string> } {
      const now = Date.now();
      const prev = prevIdsRef.current;
      const recs: MarkerRec[] = [];
      const ids = new Set<string>();
      for (const s of signals) {
        if (!s.country) continue;
        // Most precise location available: city → state → country capital. Precise
        // hits are cached; the capital fallback is recomputed until the geocoding
        // DB loads (then markers upgrade to their city/state position).
        let pos = coordRef.current.get(s.id);
        if (!pos) {
          const g = geocode(s.country, s.region, s.city);
          if (g) {
            pos = jitter(g.lat, g.lng, s.id);
            coordRef.current.set(s.id, pos);
          } else {
            const c = coords.get(s.country);
            if (c && (c.capitalLat || c.capitalLng)) pos = jitter(c.capitalLat, c.capitalLng, s.id);
          }
        }
        if (!pos) continue;
        const category = domainOf(s.eventType);
        recs.push({
          id: s.id, lat: pos[0], lng: pos[1], title: s.title, color: categoryColor(category),
          country: s.country, category, leaf: s.eventType ?? "",
          severity: s.severity, opacity: recencyOpacity(s.lastSeenAt, now, windowMs),
          breaking: isBreaking(s.severity), isNew: flagNew && !prev.has(s.id),
        });
        ids.add(s.id);
      }
      return { recs, ids };
    }

    async function paintFromCache() {
      const cached = await getCached(Date.now() - windowMs);
      if (!active || cached.length === 0) return;
      const { recs } = buildRecs(cached, false); // cache paint never ripples
      setMarkers(recs);
    }

    async function poll() {
      const since = new Date(Date.now() - windowMs).toISOString();
      let signals;
      try {
        signals = await api.liveSignals(since, country, MAX_MARKERS);
      } catch {
        return; // transient; keep polling
      }
      if (!active) return;
      const { recs, ids } = buildRecs(signals, !first);
      if (!active) return;
      const breaking = newBreaking(signals, prevIdsRef.current, first);
      const freshly = recs.filter((r) => r.isNew).length;
      prevIdsRef.current = ids;
      first = false;
      setMarkers(recs);
      if (freshly) setLastUpdate(new Date().toLocaleTimeString());
      if (breaking.length) notifyBreaking(breaking, setSelectedId);
      void mergeCached(signals); // keep the offline/instant-load cache fresh
    }

    void paintFromCache().then(() => poll());
    const t = setInterval(() => void poll(), POLL_MS);
    return () => {
      active = false;
      clearInterval(t);
    };
  }, [list, windowMin, country]);

  const sel = country ? byCode[country] : null;
  const center: [number, number] = sel && (sel.capitalLat || sel.capitalLng) ? [sel.capitalLat, sel.capitalLng] : WORLD_CENTER;
  const zoom = sel ? 5 : 2;

  const inCountry = country ? markers.filter((m) => m.country === country) : markers;
  const domainCount: Record<string, number> = {};
  const leafCount: Record<string, number> = {};
  for (const m of inCountry) {
    domainCount[m.category] = (domainCount[m.category] ?? 0) + 1;
    if (m.leaf) leafCount[m.leaf] = (leafCount[m.leaf] ?? 0) + 1;
  }
  const shown = inCountry.filter((m) => !disabledDomains.has(m.category) && !disabledLeaves.has(m.leaf));

  return (
    <div style={{ height: "calc(100dvh - 56px)", display: "flex", flexDirection: "column" }} data-testid="live-dashboard">
      <Group justify="space-between" px="md" py="xs" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
        <Group gap="xs">
          <Badge color="blue" variant="light" leftSection={<IconBroadcast size={12} />} data-testid="live-indicator">Live</Badge>
          {!online && (
            <Badge color="orange" variant="light" leftSection={<IconWifiOff size={12} />} data-testid="live-offline">Offline</Badge>
          )}
          <Text size="sm" c="dimmed">{shown.length} events on map{lastUpdate && ` · updated ${lastUpdate}`}</Text>
        </Group>
        <Group gap="xs">
          <SegmentedControl size="xs" data={VIEWS} value={view} onChange={setView} data-testid="live-view" />
          <Select data={WINDOWS} value={windowMin} onChange={(v) => setWindowMin(v ?? "60")} allowDeselect={false} w={150} data-testid="live-window" />
          <CountrySelect placeholder="Whole world" value={country} onChange={setCountry} data-testid="live-country" />
        </Group>
      </Group>
      <div style={{ position: "relative", flex: 1, minHeight: 0 }}>
        <LiveMap markers={shown} center={center} zoom={zoom} height="100%" onSelect={setSelectedId} focus={country} mode={view} />
        <Paper
          withBorder
          shadow="md"
          radius="md"
          p="xs"
          style={{ position: "absolute", top: 12, right: 12, zIndex: 1000, width: 240, maxHeight: "calc(100% - 24px)", overflowY: "auto" }}
          data-testid="live-legend"
        >
          <Group justify="space-between" wrap="nowrap" mb={legendOpen ? 6 : 0}>
            <Group gap={6} wrap="nowrap">
              <IconStack2 size={15} />
              <Text size="xs" fw={700}>Category layers</Text>
            </Group>
            <ActionIcon variant="subtle" size="sm" onClick={() => setLegendOpen((o) => !o)} aria-label="Toggle layers" data-testid="legend-toggle">
              {legendOpen ? <IconChevronUp size={15} /> : <IconChevronDown size={15} />}
            </ActionIcon>
          </Group>
          {legendOpen && (
            <Stack gap={2}>
              {CATEGORIES.map((c) => {
                const children = leavesByDomain.get(c.code) ?? [];
                const domainOff = disabledDomains.has(c.code);
                const offKids = children.filter((ch) => disabledLeaves.has(ch.code)).length;
                const isExpanded = expanded.has(c.code);
                return (
                  <div key={c.code}>
                    <Group gap={4} wrap="nowrap" justify="space-between">
                      <Checkbox
                        size="xs"
                        checked={!domainOff && offKids === 0}
                        indeterminate={!domainOff && offKids > 0}
                        onChange={() => toggleDomain(c.code)}
                        color={c.color}
                        data-testid={`layer-${c.code}`}
                        label={
                          <Group gap={6} wrap="nowrap">
                            <ColorSwatch color={c.color} size={10} />
                            <Text size="xs">{categoryLabel(c.code)}</Text>
                            <Text size="xs" c="dimmed">{domainCount[c.code] ?? 0}</Text>
                          </Group>
                        }
                      />
                      {children.length > 0 && (
                        <ActionIcon variant="subtle" size="xs" color="gray" onClick={() => toggleExpand(c.code)} aria-label={`Expand ${c.label}`} data-testid={`expand-${c.code}`}>
                          {isExpanded ? <IconChevronUp size={13} /> : <IconChevronDown size={13} />}
                        </ActionIcon>
                      )}
                    </Group>
                    {isExpanded && children.length > 0 && (
                      <Stack gap={2} pl={26} mt={2}>
                        {children.map((ch) => (
                          <Checkbox
                            key={ch.code}
                            size="xs"
                            color={c.color}
                            checked={!domainOff && !disabledLeaves.has(ch.code)}
                            disabled={domainOff}
                            onChange={() => toggleLeaf(ch.code)}
                            data-testid={`sub-${ch.code}`}
                            label={
                              <Group gap={6} wrap="nowrap">
                                <Text size="xs">{ch.label}</Text>
                                <Text size="xs" c="dimmed">{leafCount[ch.code] ?? 0}</Text>
                              </Group>
                            }
                          />
                        ))}
                      </Stack>
                    )}
                  </div>
                );
              })}
            </Stack>
          )}
        </Paper>
      </div>
      <SignalDrawer signalId={selectedId} onClose={() => setSelectedId(null)} />
    </div>
  );
}
