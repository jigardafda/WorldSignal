import { forwardRef, StrictMode, useImperativeHandle } from "react";
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
  polygonCapColor?: (d: unknown) => string;
}
const { rendererStub } = vi.hoisted(() => ({ rendererStub: { forceContextLoss: vi.fn(), dispose: vi.fn() } }));
vi.mock("react-globe.gl", () => {
  const Globe = forwardRef<unknown, GlobeProps>((props, ref) => {
    useImperativeHandle(ref, () => ({ controls: () => ({}), pointOfView: vi.fn(), renderer: () => rendererStub }));
    return (
      <div
        data-testid="globe-canvas"
        data-points={props.pointsData.length}
        data-arcs={props.arcsData.length}
        data-rings={props.ringsData.length}
        data-polygons={props.polygonsData.length}
        data-cap0={props.polygonsData[0] ? props.polygonCapColor?.(props.polygonsData[0]) : ""}
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

  it("releases the WebGL context after a real unmount (mobile context-leak guard)", async () => {
    rendererStub.forceContextLoss.mockClear();
    rendererStub.dispose.mockClear();
    const { unmount } = render(<LiveGlobe markers={[marker("a")]} />);
    unmount();
    await new Promise((r) => setTimeout(r, 0)); // release is deferred a macrotask
    expect(rendererStub.forceContextLoss).toHaveBeenCalled();
    expect(rendererStub.dispose).toHaveBeenCalled();
  });

  it("keeps the context alive across a StrictMode remount (no black globe in dev)", async () => {
    // StrictMode mounts→unmounts→remounts; force-losing the context on that fake
    // unmount would leave the reused renderer dead. The deferred release must be
    // cancelled by the remount.
    rendererStub.forceContextLoss.mockClear();
    render(
      <StrictMode>
        <LiveGlobe markers={[marker("a")]} />
      </StrictMode>,
    );
    await new Promise((r) => setTimeout(r, 0));
    expect(rendererStub.forceContextLoss).not.toHaveBeenCalled();
  });

  it("colors country polygons from the choropleth fill and hides points", async () => {
    render(<LiveGlobe markers={[marker("a")]} polygonFill={(alpha2) => (alpha2 === "US" ? "#123456" : null)} hidePoints />);
    const canvas = screen.getByTestId("globe-canvas");
    expect(canvas).toHaveAttribute("data-points", "0"); // points hidden in choropleth sub-mode
    // The US polygon (feature id 840 → alpha2 US) gets the fill color once boundaries load.
    await waitFor(() => expect(canvas.getAttribute("data-cap0")).toBe("#123456"));
  });
});
