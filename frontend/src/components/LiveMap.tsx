import { useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import "./LiveMap.css";

export interface MapMarker {
  id: string;
  lat: number;
  lng: number;
  title: string;
  color?: string;
  isNew?: boolean;
}

/** An interactive 2D world map (Leaflet + OpenStreetMap tiles) that plots events
 * as glowing pulse markers, color-coded per category. `center`/`zoom` drive
 * world-vs-country framing; markers flagged `isNew` ripple. Clicking a marker
 * invokes `onSelect` with its id. Real Leaflet runs in the browser; tests mock it. */
export function LiveMap({
  markers,
  center,
  zoom,
  height = "100%",
  onSelect,
}: {
  markers: MapMarker[];
  center: [number, number];
  zoom: number;
  height?: number | string;
  onSelect?: (id: string) => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<L.Map | null>(null);
  const layerRef = useRef<L.LayerGroup | null>(null);

  // Initialise the map exactly once.
  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;
    const map = L.map(containerRef.current, { worldCopyJump: true, minZoom: 2 }).setView(center, zoom);
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 18,
      attribution: "&copy; OpenStreetMap contributors",
    }).addTo(map);
    layerRef.current = L.layerGroup().addTo(map);
    mapRef.current = map;
    return () => {
      map.remove();
      mapRef.current = null;
      layerRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Reframe to world / selected country when center or zoom changes.
  useEffect(() => {
    mapRef.current?.setView(center, zoom);
  }, [center, zoom]);

  // Re-plot markers whenever the set changes.
  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    layer.clearLayers();
    for (const m of markers) {
      const color = (m.color ?? "#2f6df6").replace(/[^#a-zA-Z0-9(),.% ]/g, "");
      const icon = L.divIcon({
        className: "ws-marker",
        html: `<span class="ws-pulse${m.isNew ? " ws-pulse-new" : ""}" style="--ws-c:${color}"></span>`,
        iconSize: [14, 14],
        iconAnchor: [7, 7],
      });
      L.marker([m.lat, m.lng], { icon, title: m.title })
        .on("click", () => onSelect?.(m.id))
        .addTo(layer);
    }
  }, [markers, onSelect]);

  return <div ref={containerRef} data-testid="live-map" style={{ height, width: "100%", borderRadius: 8, overflow: "hidden" }} />;
}
