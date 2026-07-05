import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { render, waitFor } from "@testing-library/react";

vi.mock("../lib/boundaries", () => ({
  countryOutline: vi.fn(async (code: string) =>
    code === "FR" ? { type: "Feature", id: "250", properties: {}, geometry: { type: "Polygon", coordinates: [] } } : null,
  ),
}));

// The Leaflet plugins attach to the real L at import time; in tests we stub them
// out and expose markerClusterGroup / heatLayer on the mocked L instead.
vi.mock("leaflet.markercluster", () => ({}));
vi.mock("leaflet.heat", () => ({}));

vi.mock("leaflet", () => {
  const mapObj: Record<string, unknown> = {};
  mapObj.setView = vi.fn(() => mapObj);
  mapObj.fitBounds = vi.fn(() => mapObj);
  mapObj.remove = vi.fn();
  mapObj.removeLayer = vi.fn();
  const groupFactory = () => {
    const o: Record<string, unknown> = { addLayer: vi.fn() };
    o.addTo = vi.fn(() => o);
    return o;
  };
  const chain = () => {
    const o: Record<string, unknown> = {};
    o.addTo = vi.fn(() => o);
    o.on = vi.fn(() => o);
    return o;
  };
  return {
    default: {
      map: vi.fn(() => mapObj),
      tileLayer: vi.fn(() => chain()),
      layerGroup: vi.fn(() => groupFactory()),
      markerClusterGroup: vi.fn(() => groupFactory()),
      heatLayer: vi.fn(() => chain()),
      marker: vi.fn(() => chain()),
      divIcon: vi.fn(() => ({})),
      geoJSON: vi.fn(() => {
        const o: Record<string, unknown> = { remove: vi.fn(), getBounds: vi.fn(() => ({})) };
        o.addTo = vi.fn(() => o);
        return o;
      }),
    },
  };
});

import L from "leaflet";
import { LiveMap, type MapMarker } from "./LiveMap";

const m = (id: string, extra: Partial<MapMarker> = {}): MapMarker => ({ id, lat: 10, lng: 20, title: id, color: "#e03131", ...extra });
const iconHtmls = () => vi.mocked(L.divIcon).mock.calls.map((c) => (c[0] as { html: string }).html);

beforeEach(() => vi.clearAllMocks());

describe("LiveMap", () => {
  it("initializes Leaflet once and plots a marker per event via a layer group", () => {
    const { rerender, unmount } = render(<LiveMap markers={[m("a"), m("b", { isNew: true })]} center={[20, 0]} zoom={2} />);
    expect(vi.mocked(L.map)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(L.tileLayer)).toHaveBeenCalled();
    expect(vi.mocked(L.layerGroup)).toHaveBeenCalled();
    expect(vi.mocked(L.marker)).toHaveBeenCalledTimes(2);
    // A "new" marker uses the pulsing icon variant, color-coded via --ws-c.
    const html = iconHtmls().join("|");
    expect(html).toContain("ws-pulse-new");
    expect(html).toContain("--ws-c:#e03131");

    // Re-rendering with more markers rebuilds the layer without re-initialising.
    rerender(<LiveMap markers={[m("a"), m("b"), m("c")]} center={[20, 0]} zoom={2} />);
    expect(vi.mocked(L.map)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(L.marker)).toHaveBeenCalledTimes(5);

    rerender(<LiveMap markers={[m("a")]} center={[48, 2]} zoom={5} />);
    unmount();
  });

  it("sizes markers by severity, fades by opacity, and marks breaking events", () => {
    render(
      <LiveMap
        markers={[m("crit", { severity: "CRITICAL", isNew: true, breaking: true, opacity: 0.5 })]}
        center={[0, 0]}
        zoom={2}
      />,
    );
    // CRITICAL ⇒ 22px icon, breaking+new ⇒ ws-pulse-breaking class.
    const iconCall = vi.mocked(L.divIcon).mock.calls.at(-1)![0] as { html: string; iconSize: [number, number] };
    expect(iconCall.iconSize).toEqual([22, 22]);
    expect(iconCall.html).toContain("--ws-s:22px");
    expect(iconCall.html).toContain("ws-pulse-breaking");
    // Recency opacity is applied to the Leaflet marker.
    const markerOpts = vi.mocked(L.marker).mock.calls.at(-1)![1] as { opacity: number };
    expect(markerOpts.opacity).toBe(0.5);
  });

  it("uses a marker-cluster group in cluster mode", () => {
    render(<LiveMap markers={[m("a"), m("b")]} center={[0, 0]} zoom={2} mode="cluster" />);
    expect(vi.mocked(L.markerClusterGroup)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(L.layerGroup)).not.toHaveBeenCalled();
    expect(vi.mocked(L.marker)).toHaveBeenCalledTimes(2);
  });

  it("renders a severity-weighted heat layer in heat mode", () => {
    render(
      <LiveMap
        markers={[m("a", { severity: "CRITICAL", opacity: 1 }), m("b", { severity: "LOW", opacity: 0.5 })]}
        center={[0, 0]}
        zoom={2}
        mode="heat"
      />,
    );
    expect(vi.mocked(L.heatLayer)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(L.marker)).not.toHaveBeenCalled();
    const points = vi.mocked(L.heatLayer).mock.calls[0][0] as [number, number, number][];
    expect(points).toHaveLength(2);
    expect(points[0][2]).toBeCloseTo(1, 5); // CRITICAL (rank 3 ⇒ (3+1)/4=1) × opacity 1
    expect(points[1][2]).toBeCloseTo(0.125, 5); // LOW (rank 0 ⇒ 1/4=0.25) × opacity 0.5
  });

  it("invokes onSelect with the marker id on click", () => {
    const onSelect = vi.fn();
    render(<LiveMap markers={[m("evt-42")]} center={[0, 0]} zoom={2} onSelect={onSelect} />);
    const markerInstance = vi.mocked(L.marker).mock.results.at(-1)!.value as Record<string, unknown>;
    const onCall = (markerInstance.on as Mock).mock.calls.find((c) => c[0] === "click");
    expect(onCall).toBeTruthy();
    (onCall![1] as () => void)();
    expect(onSelect).toHaveBeenCalledWith("evt-42");
  });

  it("outlines the selected country and clears it on deselect", async () => {
    const { rerender } = render(<LiveMap markers={[]} center={[20, 0]} zoom={2} focus="FR" />);
    await waitFor(() => expect(vi.mocked(L.geoJSON)).toHaveBeenCalled());
    const map = vi.mocked(L.map).mock.results.at(-1)!.value as Record<string, unknown>;
    expect(map.fitBounds as Mock).toHaveBeenCalled();
    const layer = vi.mocked(L.geoJSON).mock.results.at(-1)!.value as Record<string, unknown>;
    rerender(<LiveMap markers={[]} center={[20, 0]} zoom={2} focus={null} />);
    expect(layer.remove as Mock).toHaveBeenCalled();
  });

  it("does not outline when no country is focused", () => {
    render(<LiveMap markers={[]} center={[20, 0]} zoom={2} focus={null} />);
    expect(vi.mocked(L.geoJSON)).not.toHaveBeenCalled();
  });
});
