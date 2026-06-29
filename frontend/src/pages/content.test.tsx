import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    stats: vi.fn(), signals: vi.fn(), signalCount: vi.fn(), signal: vi.fn(), countries: vi.fn(),
    attributeDictionary: vi.fn(),
  },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));

import { _resetCountriesCache } from "../lib/countries";
import { Dashboard } from "./Dashboard";
import { Signals } from "./Signals";
import { SignalDetail } from "./SignalDetail";

const signal = (over = {}) => ({
  id: "sg", title: "Quake", summary: "s", whatHappened: "wh", whyItMatters: "why",
  status: "CONFIRMED", severity: "HIGH", confidence: 0.8, eventType: "DISASTER.EARTHQUAKE",
  country: "US", sourceCount: 3, firstSeenAt: "2026-01-01T00:00:00Z", lastSeenAt: "2026-01-01T00:00:00Z",
  region: "California", city: "Los Angeles", locality: null, geoScope: "LOCAL",
  sentiment: "NEGATIVE", sentimentScore: -0.6, influence: "HIGH", relevance: 0.9,
  language: "en", translated: false,
  tags: [{ code: "DISASTER.EARTHQUAKE", confidence: 0.9 }],
  sources: [{ publisher: "BBC", url: "https://bbc.example/a", publishedAt: "2026-01-01T00:00:00Z" }],
  attributes: [
    { key: "industry", valueCode: "CYBERSECURITY", valueText: "", valueNum: null, confidence: 1 },
    { key: "entity", valueCode: "ORGANIZATION", valueText: "Acme Corp", valueNum: null, confidence: 0.8 },
  ],
  ...over,
});

afterEach(() => vi.clearAllMocks());
beforeEach(() => {
  _resetCountriesCache();
  apiMock.countries.mockResolvedValue([{ code: "US", name: "United States", flag: "🇺🇸", currency: "USD", capital: "Washington, D.C.", capitalLat: 38.9, capitalLng: -77.04 }]);
  apiMock.attributeDictionary.mockResolvedValue([
    { key: "industry", label: "Industry", kind: "TAGSET", description: "", values: [{ code: "CYBERSECURITY", label: "Cybersecurity" }, { code: "BANKING", label: "Banking" }] },
  ]);
});

describe("Dashboard", () => {
  it("renders KPIs and latest signals", async () => {
    apiMock.stats.mockResolvedValue({ sources: 2, articles: 4, signals: 1, deliveriesSent: 5, deliveriesPending: 1 });
    apiMock.signals.mockResolvedValue([signal()]);
    renderWithProviders(<Dashboard />);
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    expect(screen.getByText("Articles")).toBeInTheDocument();
  });
  it("shows the error state", async () => {
    apiMock.stats.mockRejectedValue(new Error("down"));
    apiMock.signals.mockResolvedValue([]);
    renderWithProviders(<Dashboard />);
    expect(await screen.findByTestId("error")).toBeInTheDocument();
  });
});

describe("Signals", () => {
  it("lists, filters and paginates", async () => {
    apiMock.signals.mockResolvedValue([signal()]);
    apiMock.signalCount.mockResolvedValue(1);
    renderWithProviders(<Signals />);
    expect(await screen.findByText("Quake")).toBeInTheDocument();

    await userEvent.type(screen.getByTestId("signal-search"), "quake{Enter}");
    await waitFor(() => expect(apiMock.signals).toHaveBeenCalledWith(expect.objectContaining({ search: "quake" }), 25, 0));

    await userEvent.click(screen.getByText("Quake")); // row click → navigate
  });
  it("shows enrichment intel in rows", async () => {
    apiMock.signals.mockResolvedValue([signal()]);
    apiMock.signalCount.mockResolvedValue(1);
    renderWithProviders(<Signals />);
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    const intel = screen.getByTestId("signal-intel");
    expect(intel).toHaveTextContent("NEGATIVE");
    expect(intel).toHaveTextContent("Los Angeles, California");
    expect(intel).toHaveTextContent("CYBERSECURITY");
  });
  it("filters by sentiment", async () => {
    apiMock.signals.mockResolvedValue([signal()]);
    apiMock.signalCount.mockResolvedValue(1);
    renderWithProviders(<Signals />);
    await screen.findByText("Quake");
    // Mantine 9 Combobox: open via native click; options stay display:none in jsdom.
    fireEvent.click(screen.getByTestId("signal-sentiment"));
    const listbox = document.getElementById(screen.getByTestId("signal-sentiment").getAttribute("aria-controls")!)!;
    fireEvent.click(await within(listbox).findByRole("option", { name: "NEGATIVE", hidden: true }));
    await waitFor(() => expect(apiMock.signals).toHaveBeenCalledWith(expect.objectContaining({ sentiment: "NEGATIVE" }), 25, 0));
  });
  it("filters by country", async () => {
    apiMock.signals.mockResolvedValue([signal()]);
    apiMock.signalCount.mockResolvedValue(1);
    renderWithProviders(<Signals />);
    await screen.findByText("Quake");
    fireEvent.click(screen.getByTestId("signal-country"));
    const listbox = document.getElementById(screen.getByTestId("signal-country").getAttribute("aria-controls")!)!;
    fireEvent.click(await within(listbox).findByRole("option", { name: /United States/, hidden: true }));
    await waitFor(() => expect(apiMock.signals).toHaveBeenCalledWith(expect.objectContaining({ country: "US" }), 25, 0));
  });
  it("shows empty state", async () => {
    apiMock.signals.mockResolvedValue([]);
    apiMock.signalCount.mockResolvedValue(0);
    renderWithProviders(<Signals />);
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
  });
});

describe("SignalDetail", () => {
  it("renders details", async () => {
    apiMock.signal.mockResolvedValue(signal());
    renderWithProviders(<SignalDetail />, { route: "/signals/sg", path: "/signals/:id" });
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    expect(screen.getByText("Why it matters")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "BBC" })).toBeInTheDocument();
  });
  it("handles not found", async () => {
    apiMock.signal.mockResolvedValue(null);
    renderWithProviders(<SignalDetail />, { route: "/signals/x", path: "/signals/:id" });
    expect(await screen.findByText("Signal not found.")).toBeInTheDocument();
  });
  it("shows a translated badge for non-English sources", async () => {
    apiMock.signal.mockResolvedValue(signal({ language: "fr", translated: true }));
    renderWithProviders(<SignalDetail />, { route: "/signals/sg", path: "/signals/:id" });
    expect(await screen.findByTestId("signal-translated")).toHaveTextContent("Translated from French");
  });
  it("hides the translated badge for English signals", async () => {
    apiMock.signal.mockResolvedValue(signal());
    renderWithProviders(<SignalDetail />, { route: "/signals/sg", path: "/signals/:id" });
    await screen.findByText("Quake");
    expect(screen.queryByTestId("signal-translated")).toBeNull();
  });
  it("renders deep-enrichment attributes", async () => {
    apiMock.signal.mockResolvedValue(signal());
    renderWithProviders(<SignalDetail />, { route: "/signals/sg", path: "/signals/:id" });
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    expect(screen.getByTestId("signal-geo")).toHaveTextContent("Los Angeles, California");
    expect(screen.getByTestId("signal-assessment")).toHaveTextContent("Sentiment: NEGATIVE");
    expect(screen.getByTestId("signal-assessment")).toHaveTextContent("Influence: HIGH");
    expect(screen.getByTestId("signal-assessment")).toHaveTextContent("Relevance: 90%");
    expect(screen.getByTestId("signal-industries")).toHaveTextContent("CYBERSECURITY");
    expect(screen.getByTestId("signal-entities")).toHaveTextContent("Acme Corp");
  });
  it("renders empty tags/sources and omits optional sections", async () => {
    apiMock.signal.mockResolvedValue(signal({ tags: [], sources: [], whatHappened: null, whyItMatters: null, eventType: null, country: null, sentiment: null, influence: null, relevance: null, region: null, city: null, locality: null, geoScope: null, attributes: [] }));
    renderWithProviders(<SignalDetail />, { route: "/signals/sg", path: "/signals/:id" });
    expect(await screen.findByText("No linked sources.")).toBeInTheDocument();
    expect(screen.getByText("No tags.")).toBeInTheDocument();
    expect(screen.getByText("Not yet assessed.")).toBeInTheDocument();
    expect(screen.queryByTestId("signal-industries")).toBeNull();
    expect(screen.queryByText("Why it matters")).toBeNull();
  });
});
