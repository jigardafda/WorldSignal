import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: { liveSignals: vi.fn(), countries: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

// Stub the Leaflet map so the page logic is testable without a real DOM map.
vi.mock("../components/LiveMap", () => ({
  LiveMap: ({ markers, center, zoom }: { markers: { id: string }[]; center: [number, number]; zoom: number }) => (
    <div data-testid="map" data-count={markers.length} data-zoom={zoom} data-center={center.join(",")} />
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

  it("survives a feed error without crashing", async () => {
    apiMock.liveSignals.mockRejectedValue(new Error("down"));
    renderWithProviders(<LiveDashboard />);
    const map = await screen.findByTestId("map");
    expect(map).toHaveAttribute("data-count", "0");
  });
});
