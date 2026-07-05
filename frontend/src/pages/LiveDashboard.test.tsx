import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: { liveSignals: vi.fn(), countries: vi.fn(), signal: vi.fn(), taxonomy: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

// Geocoding loads a large offline DB; stub it. null => fall back to country capital.
vi.mock("../lib/geocode", () => ({ geocode: vi.fn(() => null), preloadGeo: vi.fn(() => Promise.resolve()) }));

// Control the IndexedDB cache and capture breaking toasts.
const { cacheMock, notifyMock } = vi.hoisted(() => ({
  cacheMock: { getCached: vi.fn(), mergeCached: vi.fn() },
  notifyMock: vi.fn(),
}));
// The 3D globe is lazy-loaded (three.js) — stub it so the view can be exercised.
vi.mock("../components/LiveGlobe", () => ({
  default: ({ markers, sentimentTint, polygonFill, hidePoints }: { markers: { id: string }[]; sentimentTint?: boolean; polygonFill?: unknown; hidePoints?: boolean }) => (
    <div data-testid="live-globe-mock" data-count={markers.length} data-tint={sentimentTint ? "1" : ""} data-fill={polygonFill ? "1" : ""} data-hidepoints={hidePoints ? "1" : ""} />
  ),
}));

vi.mock("../lib/signalCache", () => cacheMock);
vi.mock("@mantine/notifications", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@mantine/notifications")>();
  return { ...actual, notifications: { ...actual.notifications, show: notifyMock } };
});

// Stub the Leaflet map; expose a button to trigger onSelect for the first marker
// and reflect the active view mode.
vi.mock("../components/LiveMap", () => ({
  LiveMap: ({ markers, center, zoom, onSelect, focus, mode, flyTo, sentimentTint, regions }: { markers: { id: string }[]; center: [number, number]; zoom: number; onSelect?: (id: string) => void; focus?: string | null; mode?: string; flyTo?: { lat: number; lng: number } | null; sentimentTint?: boolean; regions?: unknown }) => (
    <div data-testid="map" data-count={markers.length} data-zoom={zoom} data-center={center.join(",")} data-focus={focus ?? ""} data-mode={mode ?? "pins"} data-flyto={flyTo ? `${flyTo.lat},${flyTo.lng}` : ""} data-tint={sentimentTint ? "1" : ""} data-regions={regions ? "1" : ""}>
      {markers[0] && <button data-testid="map-pick" onClick={() => onSelect?.(markers[0].id)}>pick</button>}
    </div>
  ),
}));

import { _resetCountriesCache } from "../lib/countries";
import { LiveDashboard } from "./LiveDashboard";

const COUNTRIES = [
  { code: "US", name: "United States", flag: "🇺🇸", currency: "USD", capital: "Washington", capitalLat: 38.9, capitalLng: -77 },
  { code: "FR", name: "France", flag: "🇫🇷", currency: "EUR", capital: "Paris", capitalLat: 48.85, capitalLng: 2.35 },
];

afterEach(() => vi.clearAllMocks());
beforeEach(() => {
  _resetCountriesCache();
  cacheMock.getCached.mockResolvedValue([]);
  cacheMock.mergeCached.mockResolvedValue(undefined);
  apiMock.countries.mockResolvedValue(COUNTRIES);
  apiMock.taxonomy.mockResolvedValue([
    { code: "DISASTER", label: "Disaster", children: [{ code: "DISASTER.EARTHQUAKE", label: "Earthquake" }, { code: "DISASTER.FLOOD", label: "Flood" }] },
    { code: "TECHNOLOGY", label: "Technology", children: [{ code: "TECHNOLOGY.AI", label: "AI" }] },
  ]);
});

describe("LiveDashboard", () => {
  it("plots a marker per geo-locatable event and starts on the world view", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      { id: "s2", title: "FR strike", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
      { id: "s3", title: "No country", country: null, severity: "LOW", eventType: null, lastSeenAt: "" },
      { id: "s4", title: "Unknown country", country: "ZZ", severity: "LOW", eventType: null, lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    // s3 (no country) and s4 (no coords) are skipped; s1 + s2 plotted.
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));
    expect(map).toHaveAttribute("data-zoom", "2"); // world
    expect(map).toHaveAttribute("data-center", "20,0");
    expect(screen.getByTestId("live-indicator")).toHaveTextContent("Live");
  });

  it("reframes and filters to a selected country", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      { id: "s2", title: "FR strike", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "2"));

    fireEvent.click(screen.getByTestId("live-country"));
    const listbox = document.getElementById(screen.getByTestId("live-country").getAttribute("aria-controls")!)!;
    fireEvent.click(await within(listbox).findByRole("option", { name: /France/, hidden: true }));

    const map = screen.getByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "1")); // only FR
    expect(map).toHaveAttribute("data-zoom", "5");
    expect(map).toHaveAttribute("data-center", "48.85,2.35");
    expect(map).toHaveAttribute("data-focus", "FR"); // country outline
    // The feed is scoped to the country server-side for an accurate picture.
    await waitFor(() => expect(apiMock.liveSignals).toHaveBeenCalledWith(expect.any(String), "FR", expect.any(Number)));
  });

  it("filters events by category layer via the legend", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      { id: "s2", title: "FR AI", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));
    expect(screen.getByTestId("live-legend")).toBeInTheDocument();

    // Turn the Disaster layer off → only the Technology event remains.
    fireEvent.click(screen.getByTestId("layer-DISASTER"));
    await waitFor(() => expect(map).toHaveAttribute("data-count", "1"));
    // Turn it back on → both return.
    fireEvent.click(screen.getByTestId("layer-DISASTER"));
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));
  });

  it("selects and clears all category layers at once", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      { id: "s2", title: "FR AI", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));

    // Clear all → every category off → nothing shows.
    fireEvent.click(screen.getByTestId("layer-all"));
    await waitFor(() => expect(map).toHaveAttribute("data-count", "0"));
    expect(screen.getByText("Select all")).toBeInTheDocument();
    // Select all → everything returns.
    fireEvent.click(screen.getByTestId("layer-all"));
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));
    expect(screen.getByText("All categories")).toBeInTheDocument();
  });

  it("drills into subcategories to filter at the leaf level", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "Quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      { id: "s2", title: "Flood", country: "US", severity: "HIGH", eventType: "DISASTER.FLOOD", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2")); // both Disaster

    fireEvent.click(screen.getByTestId("expand-DISASTER")); // reveal subcategories
    fireEvent.click(await screen.findByTestId("sub-DISASTER.FLOOD")); // hide the Flood subcategory
    await waitFor(() => expect(map).toHaveAttribute("data-count", "1")); // only Earthquake remains
  });

  it("collapses the category legend widget", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    await screen.findByTestId("live-legend");
    expect(screen.getByTestId("layer-POLITICS")).toBeInTheDocument(); // expanded by default
    fireEvent.click(screen.getByTestId("legend-toggle"));
    await waitFor(() => expect(screen.queryByTestId("layer-POLITICS")).toBeNull()); // collapsed
  });

  it("requests events within a rolling time window", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    await waitFor(() => expect(apiMock.liveSignals).toHaveBeenCalled());
    const since = apiMock.liveSignals.mock.calls[0][0];
    expect(typeof since).toBe("string");
    expect(Number.isNaN(Date.parse(since))).toBe(false); // valid ISO timestamp
  });

  it("opens the detail drawer when a marker is selected", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
    ]);
    apiMock.signal.mockResolvedValue({
      id: "s1", title: "US quake detail", summary: "Big one.", whatHappened: null, whyItMatters: null,
      status: "CONFIRMED", severity: "HIGH", confidence: 0.8, eventType: "DISASTER.EARTHQUAKE",
      country: "US", region: null, city: null, sentiment: null, influence: null, relevance: null,
      language: "en", translated: false, originalTitle: null, originalSummary: null,
      sourceCount: 1, firstSeenAt: "", lastSeenAt: "", tags: [], sources: [], attributes: [],
    });
    renderWithProviders(<LiveDashboard />);
    await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "1"));
    fireEvent.click(screen.getByTestId("map-pick"));
    expect(await screen.findByText("US quake detail")).toBeInTheDocument();
    expect(apiMock.signal).toHaveBeenCalledWith("s1");
  });

  it("initializes filters from the URL so they survive a reload", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      { id: "s2", title: "FR AI", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
      { id: "s3", title: "FR flood", country: "FR", severity: "HIGH", eventType: "DISASTER.FLOOD", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />, { route: "/live?w=30&country=FR&off=DISASTER" });
    const map = await screen.findByTestId("map");
    // country=FR (s2,s3) minus the DISASTER layer (s3) => only s2 shown, framed on France.
    await waitFor(() => expect(map).toHaveAttribute("data-count", "1"));
    expect(map).toHaveAttribute("data-center", "48.85,2.35");
    expect(map).toHaveAttribute("data-zoom", "5");
    // Controls reflect the URL.
    expect((screen.getByTestId("live-window") as HTMLInputElement).value).toBe("Last 30 min");
    expect(screen.getByTestId("layer-DISASTER")).not.toBeChecked();
    expect(screen.getByTestId("layer-TECHNOLOGY")).toBeChecked();
    // The window drives the feed query (~30 min ago).
    const since = Date.parse(apiMock.liveSignals.mock.calls[0][0]);
    expect(Date.now() - since).toBeGreaterThan(25 * 60_000);
    expect(Date.now() - since).toBeLessThan(35 * 60_000);
  });

  it("survives a feed error without crashing", async () => {
    apiMock.liveSignals.mockRejectedValue(new Error("down"));
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    expect(map).toHaveAttribute("data-count", "0");
  });

  it("switches the map view mode via the segmented control and persists it to the URL", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    expect(map).toHaveAttribute("data-mode", "pins"); // default

    fireEvent.click(screen.getByRole("radio", { name: "Heat" }));
    await waitFor(() => expect(map).toHaveAttribute("data-mode", "heat"));
  });

  it("initializes the view mode from the URL", async () => {
    apiMock.liveSignals.mockResolvedValue([]);
    renderWithProviders(<LiveDashboard />, { route: "/live?view=cluster" });
    const map = await screen.findByTestId("map");
    expect(map).toHaveAttribute("data-mode", "cluster");
  });

  it("paints instantly from the cache before the network responds", async () => {
    cacheMock.getCached.mockResolvedValue([
      { id: "c1", title: "Cached quake", country: "US", region: null, city: null, severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
    ]);
    apiMock.liveSignals.mockRejectedValue(new Error("down")); // network unavailable
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    // The cached marker is shown even though the live feed failed.
    await waitFor(() => expect(map).toHaveAttribute("data-count", "1"));
  });

  it("shows an offline indicator when connectivity drops", async () => {
    apiMock.liveSignals.mockResolvedValue([]);
    renderWithProviders(<LiveDashboard />);
    await screen.findByTestId("map");
    expect(screen.queryByTestId("live-offline")).toBeNull();
    act(() => { window.dispatchEvent(new Event("offline")); });
    expect(await screen.findByTestId("live-offline")).toBeInTheDocument();
    act(() => { window.dispatchEvent(new Event("online")); });
    await waitFor(() => expect(screen.queryByTestId("live-offline")).toBeNull());
  });

  it("renders the live ticker + pulse and flies to a marker on ticker click", async () => {
    const iso = (secsAgo: number) => new Date(Date.now() - secsAgo * 1000).toISOString();
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "CRITICAL", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: iso(5) },
      { id: "s2", title: "FR strike", country: "FR", severity: "LOW", eventType: "CONFLICT.PROTEST", lastSeenAt: iso(90) },
    ]);
    apiMock.signal.mockResolvedValue({
      id: "s1", title: "US quake detail", summary: "Big one.", whatHappened: null, whyItMatters: null,
      status: "CONFIRMED", severity: "CRITICAL", confidence: 0.9, eventType: "DISASTER.EARTHQUAKE",
      country: "US", region: null, city: null, sentiment: null, influence: null, relevance: null,
      language: "en", translated: false, originalTitle: null, originalSummary: null,
      sourceCount: 1, firstSeenAt: "", lastSeenAt: "", tags: [], sources: [], attributes: [],
    });
    renderWithProviders(<LiveDashboard />);
    await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "2"));

    // Pulse velocity counts the event seen in the last 60s (s1 only).
    expect(screen.getByTestId("pulse-velocity")).toHaveTextContent("1");
    // Ticker lists newest-first; s1 (5s ago) before s2 (90s ago).
    const ticker = screen.getByTestId("live-ticker");
    const order = within(ticker).getAllByTestId(/^ticker-/).map((el) => el.getAttribute("data-testid"));
    expect(order).toEqual(["ticker-s1", "ticker-s2"]);

    // Clicking a ticker row opens the drawer and flies the map to that marker.
    fireEvent.click(screen.getByTestId("ticker-s1"));
    expect(await screen.findByText("US quake detail")).toBeInTheDocument();
    expect(screen.getByTestId("map").getAttribute("data-flyto")).toMatch(/-?\d/); // coords set
  });

  it("enters timeline replay (freezes the window, sweeps from the start) and returns to live", async () => {
    const iso = (secsAgo: number) => new Date(Date.now() - secsAgo * 1000).toISOString();
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: iso(10) },
      { id: "s2", title: "FR AI", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: iso(20) },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2")); // live

    // Enter replay: control bar + indicator appear; playhead starts at the window
    // start, so these recent events haven't been "reached" yet ⇒ none shown.
    fireEvent.click(screen.getByTestId("replay-toggle"));
    expect(await screen.findByTestId("replay-bar")).toBeInTheDocument();
    expect(screen.getByTestId("replay-indicator")).toBeInTheDocument();
    await waitFor(() => expect(map).toHaveAttribute("data-count", "0"));

    // Exit ⇒ bar gone, live set restored.
    fireEvent.click(screen.getByTestId("replay-exit"));
    await waitFor(() => expect(screen.queryByTestId("replay-bar")).toBeNull());
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));
  });

  it("exits replay when the country filter changes", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: new Date().toISOString() },
    ]);
    renderWithProviders(<LiveDashboard />);
    await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "1"));
    fireEvent.click(screen.getByTestId("replay-toggle"));
    expect(await screen.findByTestId("replay-bar")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("live-country"));
    const listbox = document.getElementById(screen.getByTestId("live-country").getAttribute("aria-controls")!)!;
    fireEvent.click(await within(listbox).findByRole("option", { name: /France/, hidden: true }));
    await waitFor(() => expect(screen.queryByTestId("replay-bar")).toBeNull());
  });

  it("filters the map by influence (Medium+, then High only)", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "", influence: "HIGH" },
      { id: "s2", title: "FR AI", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "", influence: "MEDIUM" },
      { id: "s3", title: "US story", country: "US", severity: "LOW", eventType: "GENERAL", lastSeenAt: "", influence: null },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "3")); // all

    const pickInfluence = async (label: string) => {
      fireEvent.click(screen.getByTestId("live-influence"));
      fireEvent.click(await screen.findByRole("option", { name: label, hidden: true }));
    };
    // Medium+ ⇒ drops the null-influence one (s3) ⇒ 2 remain.
    await pickInfluence("Medium+");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));

    // High only ⇒ just s1.
    await pickInfluence("High only");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "1"));
  });

  it("toggles the sentiment tint layer on the map", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "", sentiment: "NEGATIVE" },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "1"));
    expect(map).toHaveAttribute("data-tint", ""); // off by default
    fireEvent.click(screen.getByTestId("sentiment-toggle"));
    await waitFor(() => expect(map).toHaveAttribute("data-tint", "1"));
  });

  it("switches to Regions (choropleth) mode with a metric select and legend", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "", sentiment: "NEGATIVE" },
      { id: "s2", title: "FR AI", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "", sentiment: "POSITIVE" },
    ]);
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    await waitFor(() => expect(map).toHaveAttribute("data-count", "2"));
    // No choropleth chrome in pins mode.
    expect(screen.queryByTestId("choropleth-legend")).toBeNull();
    expect(screen.queryByTestId("live-metric")).toBeNull();

    fireEvent.click(screen.getByRole("radio", { name: "Regions" }));
    await waitFor(() => expect(map).toHaveAttribute("data-mode", "regions"));
    expect(map).toHaveAttribute("data-regions", "1"); // region layer supplied
    expect(screen.getByTestId("choropleth-legend")).toHaveTextContent("Signals per country"); // count default
    expect(screen.getByTestId("live-metric")).toBeInTheDocument();

    // Switch the metric → legend updates.
    fireEvent.click(screen.getByTestId("live-metric"));
    fireEvent.click(await screen.findByRole("option", { name: "By sentiment", hidden: true }));
    await waitFor(() => expect(screen.getByTestId("choropleth-legend")).toHaveTextContent("Net sentiment"));
  });

  it("switches to the 3D Globe view (lazy) and unmounts the 2D map", async () => {
    apiMock.liveSignals.mockResolvedValue([
      { id: "s1", title: "US quake", country: "US", severity: "HIGH", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      { id: "s2", title: "FR AI", country: "FR", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
    ]);
    renderWithProviders(<LiveDashboard />);
    await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "2"));

    fireEvent.click(screen.getByRole("radio", { name: "Globe" }));
    // The lazy globe resolves and receives the same displayMarkers; the 2D map unmounts.
    const globe = await screen.findByTestId("live-globe-mock");
    expect(globe).toHaveAttribute("data-count", "2");
    expect(screen.queryByTestId("map")).toBeNull();

    // Default is the plain points globe — no choropleth fill, no legend.
    expect(globe).toHaveAttribute("data-fill", "");
    expect(screen.queryByTestId("choropleth-legend")).toBeNull();

    // Choosing a metric colors the polygons, hides points, and shows the legend.
    fireEvent.click(screen.getByTestId("globe-metric"));
    fireEvent.click(await screen.findByRole("option", { name: "By count", hidden: true }));
    await waitFor(() => expect(screen.getByTestId("live-globe-mock")).toHaveAttribute("data-fill", "1"));
    expect(screen.getByTestId("live-globe-mock")).toHaveAttribute("data-hidepoints", "1");
    expect(screen.getByTestId("choropleth-legend")).toHaveTextContent("Signals per country");
  });

  it("collapses the live pulse panel", async () => {
    apiMock.liveSignals.mockResolvedValue([]);
    renderWithProviders(<LiveDashboard />);
    await screen.findByTestId("live-pulse-panel");
    expect(screen.getByTestId("pulse-velocity")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("pulse-toggle"));
    await waitFor(() => expect(screen.queryByTestId("pulse-velocity")).toBeNull());
  });

  it("shows a compact breaking-signal alert card and caches each poll", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      apiMock.liveSignals
        .mockResolvedValueOnce([{ id: "s1", title: "Calm", country: "US", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" }])
        .mockResolvedValue([
          { id: "s1", title: "Calm", country: "US", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
          { id: "s2", title: "Major quake", country: "US", severity: "CRITICAL", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
        ]);
      renderWithProviders(<LiveDashboard />);
      await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "1"));
      expect(screen.queryByTestId("breaking-alert")).toBeNull(); // first paint never alerts

      await act(async () => { await vi.advanceTimersByTimeAsync(4000); }); // next poll
      await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "2"));
      const alert = await screen.findByTestId("breaking-alert"); // s2 (CRITICAL, new) → one card
      expect(within(alert).getByText("Major quake")).toBeInTheDocument();
      expect(cacheMock.mergeCached).toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it("mutes new breaking alerts when paused", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      apiMock.liveSignals
        .mockResolvedValueOnce([{ id: "s1", title: "Calm", country: "US", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" }])
        .mockResolvedValue([
          { id: "s1", title: "Calm", country: "US", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
          { id: "s2", title: "Major quake", country: "US", severity: "CRITICAL", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
        ]);
      renderWithProviders(<LiveDashboard />);
      await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "1"));

      await act(async () => { await vi.advanceTimersByTimeAsync(4000); });
      await screen.findByTestId("breaking-alert");

      // Pause: the card clears immediately and the muted note appears.
      fireEvent.click(screen.getByTestId("breaking-pause"));
      await waitFor(() => expect(screen.queryByTestId("breaking-alert")).toBeNull());
      expect(screen.getByTestId("breaking-paused-note")).toBeInTheDocument();

      // A fresh breaking signal on the next poll must not raise a new card.
      apiMock.liveSignals.mockResolvedValue([
        { id: "s1", title: "Calm", country: "US", severity: "LOW", eventType: "TECHNOLOGY.AI", lastSeenAt: "" },
        { id: "s3", title: "Second quake", country: "US", severity: "CRITICAL", eventType: "DISASTER.EARTHQUAKE", lastSeenAt: "" },
      ]);
      await act(async () => { await vi.advanceTimersByTimeAsync(4000); });
      await waitFor(() => expect(screen.getByTestId("map")).toHaveAttribute("data-count", "2"));
      expect(screen.queryByTestId("breaking-alert")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });
});
