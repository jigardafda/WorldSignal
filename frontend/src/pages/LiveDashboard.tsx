import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ActionIcon, Badge, Checkbox, ColorSwatch, Group, Paper, Select, Stack, Text } from "@mantine/core";
import { IconBroadcast, IconChevronDown, IconChevronUp, IconStack2 } from "@tabler/icons-react";
import { api, type TaxonomyNode } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useCountries } from "../lib/countries";
import { CountrySelect } from "../components/CountrySelect";
import { LiveMap, type MapMarker } from "../components/LiveMap";
import { SignalDrawer } from "../components/SignalDrawer";
import { jitter } from "../lib/geo";
import { geocode, preloadGeo } from "../lib/geocode";
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

  useEffect(() => {
    if (list.length === 0) return; // wait until country coordinates are loaded
    void preloadGeo(); // warm the precise-geocoding DB in the background
    const coords = new Map(list.map((c) => [c.code, c]));
    const windowMs = Number(windowMin) * 60_000;
    let active = true;
    let first = true;
    async function poll() {
      const since = new Date(Date.now() - windowMs).toISOString();
      let signals;
      try {
        signals = await api.liveSignals(since, country, MAX_MARKERS);
      } catch {
        return; // transient; keep polling
      }
      if (!active) return;
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
        recs.push({ id: s.id, lat: pos[0], lng: pos[1], title: s.title, color: categoryColor(category), country: s.country, category, leaf: s.eventType ?? "", isNew: !first && !prev.has(s.id) });
        ids.add(s.id);
      }
      if (!active) return;
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
          <Text size="sm" c="dimmed">{shown.length} events on map{lastUpdate && ` · updated ${lastUpdate}`}</Text>
        </Group>
        <Group gap="xs">
          <Select data={WINDOWS} value={windowMin} onChange={(v) => setWindowMin(v ?? "60")} allowDeselect={false} w={150} data-testid="live-window" />
          <CountrySelect placeholder="Whole world" value={country} onChange={setCountry} data-testid="live-country" />
        </Group>
      </Group>
      <div style={{ position: "relative", flex: 1, minHeight: 0 }}>
        <LiveMap markers={shown} center={center} zoom={zoom} height="100%" onSelect={setSelectedId} focus={country} />
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
