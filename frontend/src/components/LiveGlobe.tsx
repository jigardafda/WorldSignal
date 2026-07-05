import { useEffect, useMemo, useRef, useState } from "react";
import Globe, { type GlobeMethods } from "react-globe.gl";
import { allCountryOutlines } from "../lib/boundaries";
import { toArcs, toPoints, toRings, type GlobePoint, type PointInput } from "../lib/globeData";

export interface LiveGlobeProps {
  markers: PointInput[];
  onSelect?: (id: string) => void;
  sentimentTint?: boolean;
  focusLatLng?: [number, number] | null; // rotate to a selected country
  flyTo?: { lat: number; lng: number; nonce: number } | null; // rotate to a ticker-picked marker
}

const BG = "#0b1220";

/** 3D globe view (react-globe.gl / three.js). Renders the world as country
 * polygons (no external texture — CSP/offline-safe), plots live signals as
 * points (color/size/opacity mirroring the 2D map), draws the chronological
 * activity thread as arcs, and pulses breaking arrivals as rings. Auto-rotates
 * until the user interacts. Lazy-loaded, so three only ships to Globe users.
 * WebGL doesn't run in jsdom, so this is browser-verified; tests mock the lib. */
export default function LiveGlobe({ markers, onSelect, sentimentTint = false, focusLatLng = null, flyTo = null }: LiveGlobeProps) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const globeRef = useRef<GlobeMethods | undefined>(undefined);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [polygons, setPolygons] = useState<GeoJSON.Feature[]>([]);

  // Track the container size (react-globe.gl needs explicit width/height).
  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const measure = () => setSize({ w: el.clientWidth, h: el.clientHeight });
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Load the country polygons once (shared cached boundaries).
  useEffect(() => {
    let active = true;
    void allCountryOutlines().then((fmap) => {
      if (active) setPolygons([...fmap.values()]);
    });
    return () => {
      active = false;
    };
  }, []);

  // Gentle auto-rotate; three's OrbitControls stops it as soon as the user drags.
  useEffect(() => {
    const controls = globeRef.current?.controls() as { autoRotate?: boolean; autoRotateSpeed?: number } | undefined;
    if (controls) {
      controls.autoRotate = true;
      controls.autoRotateSpeed = 0.5;
    }
  }, [polygons.length]);

  // Rotate to a ticker-picked marker.
  useEffect(() => {
    if (flyTo) globeRef.current?.pointOfView({ lat: flyTo.lat, lng: flyTo.lng, altitude: 1.6 }, 800);
  }, [flyTo]);

  // Rotate to the selected country.
  useEffect(() => {
    if (focusLatLng) globeRef.current?.pointOfView({ lat: focusLatLng[0], lng: focusLatLng[1], altitude: 1.8 }, 800);
  }, [focusLatLng]);

  const points = useMemo(() => toPoints(markers, sentimentTint), [markers, sentimentTint]);
  const arcs = useMemo(() => toArcs(markers), [markers]);
  const rings = useMemo(() => toRings(markers), [markers]);

  return (
    <div ref={wrapRef} data-testid="live-globe" style={{ width: "100%", height: "100%", background: BG }}>
      <Globe
        ref={globeRef}
        width={size.w || 800}
        height={size.h || 600}
        backgroundColor={BG}
        showAtmosphere
        atmosphereColor="#3b6ea5"
        polygonsData={polygons}
        polygonAltitude={0.006}
        polygonCapColor={() => "rgba(56,78,120,0.35)"}
        polygonSideColor={() => "rgba(0,0,0,0)"}
        polygonStrokeColor={() => "rgba(150,175,220,0.45)"}
        pointsData={points}
        pointLat="lat"
        pointLng="lng"
        pointColor="color"
        pointRadius={(d) => (d as GlobePoint).size}
        pointAltitude={(d) => (d as GlobePoint).size * 0.12}
        pointLabel={(d) => (d as GlobePoint).title}
        onPointClick={(d) => onSelect?.((d as GlobePoint).id)}
        arcsData={arcs}
        arcStartLat="startLat"
        arcStartLng="startLng"
        arcEndLat="endLat"
        arcEndLng="endLng"
        arcColor="color"
        arcStroke={0.5}
        arcDashLength={0.4}
        arcDashGap={0.2}
        arcDashAnimateTime={1500}
        ringsData={rings}
        ringLat="lat"
        ringLng="lng"
        ringColor="color"
        ringMaxRadius={4}
        ringPropagationSpeed={2}
        ringRepeatPeriod={800}
      />
    </div>
  );
}
