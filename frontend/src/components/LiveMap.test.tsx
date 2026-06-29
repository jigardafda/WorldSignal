import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";

vi.mock("leaflet", () => {
  const mapObj: Record<string, unknown> = {};
  mapObj.setView = vi.fn(() => mapObj);
  mapObj.remove = vi.fn();
  const layerObj: Record<string, unknown> = { clearLayers: vi.fn() };
  layerObj.addTo = vi.fn(() => layerObj);
  const chain = () => {
    const o: Record<string, unknown> = {};
    o.addTo = vi.fn(() => o);
    o.bindPopup = vi.fn(() => o);
    return o;
  };
  return {
    default: {
      map: vi.fn(() => mapObj),
      tileLayer: vi.fn(() => chain()),
      layerGroup: vi.fn(() => layerObj),
      marker: vi.fn(() => chain()),
      divIcon: vi.fn(() => ({})),
    },
  };
});

import L from "leaflet";
import { LiveMap, type MapMarker } from "./LiveMap";

const m = (id: string, isNew = false): MapMarker => ({ id, lat: 10, lng: 20, title: id, color: "#e03131", isNew });

describe("LiveMap", () => {
  it("initializes Leaflet once and plots a marker per event", () => {
    const { rerender, unmount } = render(<LiveMap markers={[m("a"), m("b", true)]} center={[20, 0]} zoom={2} />);
    expect(vi.mocked(L.map)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(L.tileLayer)).toHaveBeenCalled();
    expect(vi.mocked(L.marker)).toHaveBeenCalledTimes(2);
    // A "new" marker uses the pulsing icon variant, color-coded via --ws-c.
    const iconHtml = vi.mocked(L.divIcon).mock.calls.map((c) => (c[0] as { html: string }).html).join("|");
    expect(iconHtml).toContain("ws-pulse-new");
    expect(iconHtml).toContain("--ws-c:#e03131");

    // Re-rendering with more markers re-plots without re-initialising the map.
    rerender(<LiveMap markers={[m("a"), m("b"), m("c")]} center={[20, 0]} zoom={2} />);
    expect(vi.mocked(L.map)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(L.marker)).toHaveBeenCalledTimes(5);

    // Changing the frame recenters the existing map.
    rerender(<LiveMap markers={[m("a")]} center={[48, 2]} zoom={5} />);
    unmount();
  });
});
