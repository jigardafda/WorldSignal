import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: { liveSignals: vi.fn(), countries: vi.fn(), signal: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

// Stub the Leaflet map; expose a button to trigger onSelect for the first marker.
vi.mock("../components/LiveMap", () => ({
  LiveMap: ({ markers, center, zoom, onSelect, focus }: { markers: { id: string }[]; center: [number, number]; zoom: number; onSelect?: (id: string) => void; focus?: string | null }) => (
    <div data-testid="map" data-count={markers.length} data-zoom={zoom} data-center={center.join(",")} data-focus={focus ?? ""}>
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
  apiMock.countries.mockResolvedValue(COUNTRIES);
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
});
