import { forwardRef, useImperativeHandle } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// react-globe.gl is a WebGL/three.js component that can't run in jsdom — stub it
// with a div that records the layer data it was handed and exposes a point click.
interface GlobeProps {
  pointsData: unknown[];
  arcsData: unknown[];
  ringsData: unknown[];
  polygonsData: unknown[];
  onPointClick?: (d: unknown) => void;
}
vi.mock("react-globe.gl", () => {
  const Globe = forwardRef<unknown, GlobeProps>((props, ref) => {
    useImperativeHandle(ref, () => ({ controls: () => ({}), pointOfView: vi.fn() }));
    return (
      <div
        data-testid="globe-canvas"
        data-points={props.pointsData.length}
        data-arcs={props.arcsData.length}
        data-rings={props.ringsData.length}
        data-polygons={props.polygonsData.length}
      >
        <button data-testid="globe-pick" onClick={() => props.onPointClick?.(props.pointsData[0])}>pick</button>
      </div>
    );
  });
  return { default: Globe };
});

vi.mock("../lib/boundaries", () => ({
  allCountryOutlines: vi.fn(async () => new Map([["US", { type: "Feature", id: "840", properties: {}, geometry: { type: "Polygon", coordinates: [] } }]])),
}));

import LiveGlobe from "./LiveGlobe";
import type { PointInput } from "../lib/globeData";

const marker = (id: string, over: Partial<PointInput> = {}): PointInput => ({ id, lat: 10, lng: 20, title: id, color: "#e03131", lastSeenMs: 100, ...over });

describe("LiveGlobe", () => {
  it("feeds points, the arc thread, and country polygons to the globe", async () => {
    render(<LiveGlobe markers={[marker("a", { lastSeenMs: 300 }), marker("b", { lastSeenMs: 200 }), marker("c", { lastSeenMs: 100 })]} />);
    const canvas = screen.getByTestId("globe-canvas");
    expect(canvas).toHaveAttribute("data-points", "3");
    expect(canvas).toHaveAttribute("data-arcs", "2"); // chronological thread over 3 events
    await waitFor(() => expect(canvas).toHaveAttribute("data-polygons", "1")); // boundaries load async
  });

  it("invokes onSelect with the clicked point's id", () => {
    const onSelect = vi.fn();
    render(<LiveGlobe markers={[marker("evt-9")]} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId("globe-pick"));
    expect(onSelect).toHaveBeenCalledWith("evt-9");
  });

  it("rings breaking arrivals", () => {
    render(<LiveGlobe markers={[marker("a", { breaking: true, isNew: true }), marker("b")]} />);
    expect(screen.getByTestId("globe-canvas")).toHaveAttribute("data-rings", "1");
  });
});
