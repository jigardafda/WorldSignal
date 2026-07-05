import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ActionIcon, Anchor, Badge, Center, Checkbox, ColorSwatch, Group, Loader, Paper, SegmentedControl, Select, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconActivity, IconAlertTriangle, IconBroadcast, IconChevronDown, IconChevronUp, IconMoodSmile, IconPlayerPlay, IconStack2, IconWifiOff } from "@tabler/icons-react";
import { api, type LiveSignal, type TaxonomyNode } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useCountries } from "../lib/countries";
import { CountrySelect } from "../components/CountrySelect";
import { LiveMap, type MapMarker, type MapMode, type RegionLayer } from "../components/LiveMap";
import { LivePulse } from "../components/LivePulse";
import { LiveTicker } from "../components/LiveTicker";
import { ReplayBar } from "../components/ReplayBar";
import { ChoroplethLegend } from "../components/ChoroplethLegend";
import { SignalDrawer } from "../components/SignalDrawer";
import { aggregateByCountry, fillFor, metricMax, metricValue, type Metric } from "../lib/choropleth";
import { countryDisplay } from "../lib/countries";
import { jitter } from "../lib/geo";
import { geocode, preloadGeo } from "../lib/geocode";
import { CATEGORIES, categoryColor, categoryLabel, domainOf } from "../lib/categories";
import { influenceRank, isBreaking, newBreaking, recencyOpacity } from "../lib/liveMarkers";
import { frameMarkers } from "../lib/replay";
import { useReplay } from "../lib/useReplay";
import { getCached, mergeCached } from "../lib/signalCache";

const POLL_MS = 4000;
const MAX_MARKERS = 2000;
const WORLD_CENTER: [number, number] = [20, 0];

// three.js is heavy — only load the globe when the user opens that view.
const LiveGlobe = lazy(() => import("../components/LiveGlobe"));

type View = MapMode | "globe";
const VIEWS: { value: View; label: string }[] = [
  { value: "pins", label: "Pins" },
  { value: "cluster", label: "Clusters" },
  { value: "heat", label: "Heat" },
  { value: "regions", label: "Regions" },
  { value: "globe", label: "Globe" },
];
const isView = (v: string | null): v is View => v === "cluster" || v === "heat" || v === "regions" || v === "globe";

const METRIC_OPTIONS = [
  { value: "count", label: "By count" },
  { value: "severity", label: "By severity" },
  { value: "sentiment", label: "By sentiment" },
];
const GLOBE_METRIC_OPTIONS = [{ value: "points", label: "Points" }, ...METRIC_OPTIONS];
const isMetric = (v: string | null): v is Metric => v === "count" || v === "severity" || v === "sentiment";

const INFLUENCE_OPTIONS = [
  { value: "all", label: "All influence" },
  { value: "MEDIUM", label: "Medium+" },
  { value: "HIGH", label: "High only" },
];

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

type MarkerRec = MapMarker & { country: string; category: string; leaf: string; lastSeenMs: number; influence?: string | null };

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
  const offParam = params.get("off") ?? "";
  const offsubParam = params.get("offsub") ?? "";
  const disabledDomains = useMemo(() => new Set(offParam.split(",").filter(Boolean)), [offParam]);
  const disabledLeaves = useMemo(() => new Set(offsubParam.split(",").filter(Boolean)), [offsubParam]);

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
  const view: View = isView(params.get("view")) ? (params.get("view") as View) : "pins";
  const setView = (v: string) => setOne("view", v === "pins" ? "" : v); // pins = default → omit
  const sentimentTint = params.get("sent") === "1";
  const setSentimentTint = (on: boolean) => setOne("sent", on ? "1" : "");
  const influence = params.get("infl") ?? ""; // "" (all) | "MEDIUM" | "HIGH"
  const setInfluence = (v: string) => setOne("infl", v === "all" ? "" : v);
  const minInfluenceRank = influenceRank(influence);
  const metric: Metric = isMetric(params.get("metric")) ? (params.get("metric") as Metric) : "count";
  const setMetric = (v: string) => setOne("metric", v === "count" ? "" : v); // count = default → omit
  // Globe choropleth: null ⇒ plain points globe; a metric ⇒ color country polygons.
  const globeMetric: Metric | null = isMetric(params.get("gmetric")) ? (params.get("gmetric") as Metric) : null;
  const setGlobeMetric = (v: string) => setOne("gmetric", v === "points" ? "" : v); // points = default → omit
  // Stable refs so the memoized choropleth layer's click handler and tooltip use
  // the latest country setter / lookup without re-running the memo each render.
  const selectCountryRef = useRef<(v: string | null) => void>(() => {});
  const byCodeRef = useRef(byCode);
  useEffect(() => {
    selectCountryRef.current = (v) => setOne("country", v ?? "");
    byCodeRef.current = byCode;
  });

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
  const [pulseOpen, setPulseOpen] = useState(true);
  const [flyTo, setFlyTo] = useState<{ lat: number; lng: number; nonce: number } | null>(null);
  const flyNonce = useRef(0);
  // Clicking a ticker row opens the signal and flies the map to its marker.
  const pickMarker = (r: MarkerRec) => {
    setSelectedId(r.id);
    setFlyTo({ lat: r.lat, lng: r.lng, nonce: ++flyNonce.current });
  };

  // Timeline replay: entering freezes the loaded window (`end` = now) and pauses
  // polling; the playhead sweeps [end − window, end]. Changing window/country
  // exits replay (they need a fresh fetch).
  const [replay, setReplay] = useState<{ on: boolean; end: number }>({ on: false, end: 0 });
  const replayWindowMs = Number(windowMin) * 60_000;
  const replayCtl = useReplay(replay.end - replayWindowMs, replay.end, replay.on);
  const toggleReplay = () => setReplay((r) => (r.on ? { ...r, on: false } : { on: true, end: Date.now() }));
  useEffect(() => { setReplay((r) => (r.on ? { ...r, on: false } : r)); }, [windowMin, country]);
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
    if (list.length === 0 || replay.on) return; // paused while replaying (frozen window)
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
          lastSeenMs: Date.parse(s.lastSeenAt ?? "") || 0,
          sourceCount: s.sourceCount, sentiment: s.sentiment, influence: s.influence,
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
  }, [list, windowMin, country, replay.on]);

  const sel = country ? byCode[country] : null;
  const center: [number, number] = sel && (sel.capitalLat || sel.capitalLng) ? [sel.capitalLat, sel.capitalLng] : WORLD_CENTER;
  const zoom = sel ? 5 : 2;

  const inCountry = useMemo(() => (country ? markers.filter((m) => m.country === country) : markers), [markers, country]);
  const domainCount: Record<string, number> = {};
  const leafCount: Record<string, number> = {};
  for (const m of inCountry) {
    domainCount[m.category] = (domainCount[m.category] ?? 0) + 1;
    if (m.leaf) leafCount[m.leaf] = (leafCount[m.leaf] ?? 0) + 1;
  }
  const shown = useMemo(
    () =>
      inCountry.filter(
        (m) =>
          !disabledDomains.has(m.category) &&
          !disabledLeaves.has(m.leaf) &&
          (minInfluenceRank === 0 || influenceRank(m.influence) >= minInfluenceRank),
      ),
    [inCountry, disabledDomains, disabledLeaves, minInfluenceRank],
  );

  // Choropleth layer (only in Regions mode): aggregate the visible set by country
  // and derive per-country fill/tooltip. Memoized so it rebuilds only on data or
  // metric change, not on every render (it repaints ~180 polygons).
  const regions = useMemo<RegionLayer | null>(() => {
    if (view !== "regions") return null;
    const agg = aggregateByCountry(shown);
    const max = metricMax(agg.values(), metric);
    const label = (a: string) => countryDisplay(a, byCodeRef.current);
    const valueText = (v: number) =>
      metric === "count" ? `${v}` : metric === "severity" ? `${Math.round(v * 100)}% high` : `${v > 0 ? "+" : ""}${v.toFixed(2)}`;
    return {
      fill: (a) => {
        const x = agg.get(a);
        return x ? fillFor(metricValue(x, metric), metric, max) : null;
      },
      tooltip: (a) => {
        const x = agg.get(a);
        return x ? `${label(a)} — ${x.count} signal${x.count === 1 ? "" : "s"}, ${valueText(metricValue(x, metric))}` : label(a);
      },
      onSelect: (a) => selectCountryRef.current(a),
    };
  }, [view, shown, metric]);

  // Globe choropleth fill: alpha-2 → hex, from the same metric aggregation.
  const globePolygonFill = useMemo<((alpha2: string) => string | null) | null>(() => {
    if (view !== "globe" || !globeMetric) return null;
    const agg = aggregateByCountry(shown);
    const max = metricMax(agg.values(), globeMetric);
    return (alpha2) => {
      const x = agg.get(alpha2);
      return x ? fillFor(metricValue(x, globeMetric), globeMetric, max) : null;
    };
  }, [view, shown, globeMetric]);

  const windowMs = Number(windowMin) * 60_000;
  // During replay the map shows the frozen window up to the playhead; otherwise
  // the live set. Layer/country filters apply either way (they shape `shown`).
  const displayMarkers = replay.on ? frameMarkers(shown, replayCtl.playheadMs, windowMs, replayCtl.prevPlayheadMs) : shown;
  const focusLatLng: [number, number] | null = sel && (sel.capitalLat || sel.capitalLng) ? [sel.capitalLat, sel.capitalLng] : null;

  return (
    <div style={{ height: "calc(100dvh - 56px)", display: "flex", flexDirection: "column" }} data-testid="live-dashboard">
      <Group justify="space-between" px="md" py="xs" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
        <Group gap="xs">
          <Badge color="blue" variant="light" leftSection={<IconBroadcast size={12} />} data-testid="live-indicator">Live</Badge>
          {replay.on && (
            <Badge color="grape" variant="light" leftSection={<IconPlayerPlay size={12} />} data-testid="replay-indicator">Replay</Badge>
          )}
          {!online && (
            <Badge color="orange" variant="light" leftSection={<IconWifiOff size={12} />} data-testid="live-offline">Offline</Badge>
          )}
          <Text size="sm" c="dimmed">{displayMarkers.length} events on map{!replay.on && lastUpdate && ` · updated ${lastUpdate}`}</Text>
        </Group>
        <Group gap="xs">
          <ActionIcon
            variant={replay.on ? "filled" : "default"}
            color="grape"
            size="lg"
            onClick={toggleReplay}
            disabled={markers.length === 0}
            aria-label="Timeline replay"
            data-testid="replay-toggle"
          >
            <IconPlayerPlay size={16} />
          </ActionIcon>
          <ActionIcon
            variant={sentimentTint ? "filled" : "default"}
            color="teal"
            size="lg"
            onClick={() => setSentimentTint(!sentimentTint)}
            aria-label="Toggle sentiment tint"
            title="Color marker borders by sentiment"
            data-testid="sentiment-toggle"
          >
            <IconMoodSmile size={16} />
          </ActionIcon>
          <SegmentedControl size="xs" data={VIEWS} value={view} onChange={setView} data-testid="live-view" />
          {view === "regions" && (
            <Select data={METRIC_OPTIONS} value={metric} onChange={(v) => setMetric(v ?? "count")} allowDeselect={false} w={140} data-testid="live-metric" />
          )}
          {view === "globe" && (
            <Select data={GLOBE_METRIC_OPTIONS} value={globeMetric ?? "points"} onChange={(v) => setGlobeMetric(v ?? "points")} allowDeselect={false} w={140} data-testid="globe-metric" />
          )}
          <Select data={INFLUENCE_OPTIONS} value={influence || "all"} onChange={(v) => setInfluence(v ?? "all")} allowDeselect={false} w={140} data-testid="live-influence" />
          <Select data={WINDOWS} value={windowMin} onChange={(v) => setWindowMin(v ?? "60")} allowDeselect={false} w={150} data-testid="live-window" />
          <CountrySelect placeholder="Whole world" value={country} onChange={setCountry} data-testid="live-country" />
        </Group>
      </Group>
      <div style={{ position: "relative", flex: 1, minHeight: 0 }}>
        {view === "globe" ? (
          <Suspense fallback={<Center h="100%" data-testid="globe-loading"><Loader /></Center>}>
            <LiveGlobe markers={displayMarkers} onSelect={setSelectedId} sentimentTint={sentimentTint} focusLatLng={focusLatLng} flyTo={flyTo} polygonFill={globePolygonFill} hidePoints={!!globePolygonFill} />
          </Suspense>
        ) : (
          <LiveMap markers={displayMarkers} center={center} zoom={zoom} height="100%" onSelect={setSelectedId} focus={country} mode={view} flyTo={flyTo} sentimentTint={sentimentTint} regions={regions} />
        )}
        {view === "regions" && <ChoroplethLegend metric={metric} max={metricMax(aggregateByCountry(shown).values(), metric)} />}
        {view === "globe" && globeMetric && <ChoroplethLegend metric={globeMetric} max={metricMax(aggregateByCountry(shown).values(), globeMetric)} />}
        {replay.on && (
          <ReplayBar
            playing={replayCtl.playing}
            playheadMs={replayCtl.playheadMs}
            progress={replayCtl.progress}
            speed={replayCtl.speed}
            atEnd={replayCtl.atEnd}
            onPlayPause={() => (replayCtl.playing ? replayCtl.pause() : replayCtl.play())}
            onSeek={replayCtl.seekProgress}
            onCycleSpeed={replayCtl.cycleSpeed}
            onExit={toggleReplay}
          />
        )}
        <Paper
          withBorder
          shadow="md"
          radius="md"
          p="xs"
          style={{ position: "absolute", top: 12, left: 12, zIndex: 1000, width: 260, maxHeight: "calc(100% - 24px)", display: "flex", flexDirection: "column" }}
          data-testid="live-pulse-panel"
        >
          <Group justify="space-between" wrap="nowrap" mb={pulseOpen ? 6 : 0}>
            <Group gap={6} wrap="nowrap">
              <IconActivity size={15} />
              <Text size="xs" fw={700}>Live pulse</Text>
            </Group>
            <ActionIcon variant="subtle" size="sm" onClick={() => setPulseOpen((o) => !o)} aria-label="Toggle pulse" data-testid="pulse-toggle">
              {pulseOpen ? <IconChevronUp size={15} /> : <IconChevronDown size={15} />}
            </ActionIcon>
          </Group>
          {pulseOpen && (
            <>
              <LivePulse recs={inCountry} windowMs={windowMs} byCode={byCode} />
              <Text size="xs" tt="uppercase" c="dimmed" fw={700} mt={8} mb={2}>Latest</Text>
              <div style={{ overflowY: "auto", minHeight: 0 }}>
                <LiveTicker recs={shown} onPick={pickMarker} />
              </div>
            </>
          )}
        </Paper>
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
