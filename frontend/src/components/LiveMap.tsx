import { useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import "leaflet.markercluster";
import "leaflet.markercluster/dist/MarkerCluster.css";
import "leaflet.markercluster/dist/MarkerCluster.Default.css";
import "leaflet.heat";
import "./LiveMap.css";
import { countryOutline } from "../lib/boundaries";
import { markerSize, severityRank } from "../lib/liveMarkers";

export type MapMode = "pins" | "cluster" | "heat";

export interface MapMarker {
  id: string;
  lat: number;
  lng: number;
  title: string;
  color?: string;
  isNew?: boolean;
  /** Severity code (LOW|MEDIUM|HIGH|CRITICAL) — drives size and heat weight. */
  severity?: string | null;
  /** Recency fade, 0..1. Defaults to fully opaque. */
  opacity?: number;
  /** New + high-severity ⇒ a stronger "breaking" ripple. */
  breaking?: boolean;
}

const COLOR_RE = /[^#a-zA-Z0-9(),.% ]/g;

function buildMarker(m: MapMarker, onSelect?: (id: string) => void): L.Marker {
  const size = markerSize(m.severity);
  const color = (m.color ?? "#2f6df6").replace(COLOR_RE, "");
  const cls = `ws-pulse${m.isNew ? " ws-pulse-new" : ""}${m.breaking && m.isNew ? " ws-pulse-breaking" : ""}`;
  const icon = L.divIcon({
    className: "ws-marker",
    html: `<span class="${cls}" style="--ws-c:${color};--ws-s:${size}px"></span>`,
    iconSize: [size, size],
    iconAnchor: [size / 2, size / 2],
  });
  return L.marker([m.lat, m.lng], { icon, title: m.title, opacity: m.opacity ?? 1 }).on("click", () => onSelect?.(m.id));
}

/** Heat intensity: severity (0.25..1) attenuated by recency opacity. */
function heatWeight(m: MapMarker): number {
  return ((severityRank(m.severity) + 1) / 4) * (m.opacity ?? 1);
}

/** An interactive 2D world map (Leaflet + OpenStreetMap tiles). In `pins`/`cluster`
 * mode it plots events as glowing pulse markers sized by severity and faded by
 * recency; `cluster` groups dense areas; `heat` renders a severity-weighted
 * density surface. `center`/`zoom` drive world-vs-country framing. Clicking a
 * marker invokes `onSelect`. Real Leaflet runs in the browser; tests mock it. */
export function LiveMap({
  markers,
  center,
  zoom,
  height = "100%",
  onSelect,
  focus = null,
  mode = "pins",
}: {
  markers: MapMarker[];
  center: [number, number];
  zoom: number;
  height?: number | string;
  onSelect?: (id: string) => void;
  focus?: string | null; // ISO alpha-2 country code to outline, or null
  mode?: MapMode;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<L.Map | null>(null);
  const displayRef = useRef<L.Layer | null>(null);
  const focusRef = useRef<L.GeoJSON | null>(null);

  // Initialise the map exactly once.
  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;
    const map = L.map(containerRef.current, { worldCopyJump: true, minZoom: 2 }).setView(center, zoom);
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 18,
      attribution: "&copy; OpenStreetMap contributors",
    }).addTo(map);
    mapRef.current = map;
    return () => {
      map.remove();
      mapRef.current = null;
      displayRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Reframe to world / selected country when center or zoom changes.
  useEffect(() => {
    mapRef.current?.setView(center, zoom);
  }, [center, zoom]);

  // Outline the selected country's actual border (lazy-loaded boundaries) and
  // frame the map to it. Clears when no country is selected.
  useEffect(() => {
    let cancelled = false;
    if (focusRef.current) {
      focusRef.current.remove();
      focusRef.current = null;
    }
    if (focus) {
      void countryOutline(focus).then((feat) => {
        const map = mapRef.current;
        if (cancelled || !map || !feat) return;
        const layer = L.geoJSON(feat, {
          style: { color: "#2f6df6", weight: 2, fillColor: "#2f6df6", fillOpacity: 0.08 },
        }).addTo(map);
        focusRef.current = layer;
        try {
          map.fitBounds(layer.getBounds(), { maxZoom: 6, padding: [20, 20] });
        } catch {
          /* empty/invalid bounds — keep the capital-centered view */
        }
      });
    }
    return () => {
      cancelled = true;
    };
  }, [focus]);

  // Rebuild the display layer whenever the markers or the view mode change.
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    if (displayRef.current) {
      map.removeLayer(displayRef.current);
      displayRef.current = null;
    }
    if (mode === "heat") {
      const points = markers.map((m) => [m.lat, m.lng, heatWeight(m)] as [number, number, number]);
      displayRef.current = L.heatLayer(points, { radius: 25, blur: 18, maxZoom: 8, minOpacity: 0.3 }).addTo(map);
      return;
    }
    const group = mode === "cluster" ? L.markerClusterGroup({ chunkedLoading: true, maxClusterRadius: 50 }) : L.layerGroup();
    for (const m of markers) group.addLayer(buildMarker(m, onSelect));
    displayRef.current = group.addTo(map);
  }, [markers, mode, onSelect]);

  return <div ref={containerRef} data-testid="live-map" style={{ height, width: "100%", borderRadius: 8, overflow: "hidden" }} />;
}
