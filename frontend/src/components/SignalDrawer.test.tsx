import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock } = vi.hoisted(() => ({ apiMock: { signal: vi.fn(), countries: vi.fn() } }));
vi.mock("../lib/api", () => ({ api: apiMock }));

import { _resetCountriesCache } from "../lib/countries";
import { SignalDrawer } from "./SignalDrawer";

const signal = (over = {}) => ({
  id: "sg", title: "Quake hits coast", summary: "A strong quake.", whatHappened: "wh", whyItMatters: "why",
  status: "CONFIRMED", severity: "HIGH", confidence: 0.8, eventType: "DISASTER.EARTHQUAKE",
  country: "US", region: "California", city: "LA", sentiment: "NEGATIVE", influence: "HIGH", relevance: 0.9,
  language: "en", translated: false, originalTitle: null, originalSummary: null,
  sourceCount: 1, firstSeenAt: "", lastSeenAt: "",
  tags: [{ code: "DISASTER.EARTHQUAKE", confidence: 0.9 }],
  sources: [{ publisher: "BBC", url: "https://bbc.example/a", publishedAt: null }],
  attributes: [],
  ...over,
});

afterEach(() => vi.clearAllMocks());
beforeEach(() => {
  _resetCountriesCache();
  apiMock.countries.mockResolvedValue([{ code: "US", name: "United States", flag: "🇺🇸", currency: "USD", capital: "Washington", capitalLat: 38.9, capitalLng: -77 }]);
});

describe("SignalDrawer", () => {
  it("fetches and shows signal details when opened", async () => {
    apiMock.signal.mockResolvedValue(signal());
    renderWithProviders(<SignalDrawer signalId="sg" onClose={() => {}} />);
    expect(await screen.findByText("Quake hits coast")).toBeInTheDocument();
    expect(screen.getByText("A strong quake.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "BBC" })).toBeInTheDocument();
    expect(apiMock.signal).toHaveBeenCalledWith("sg");
  });

  it("does not fetch when closed (no signalId)", () => {
    renderWithProviders(<SignalDrawer signalId={null} onClose={() => {}} />);
    expect(apiMock.signal).not.toHaveBeenCalled();
  });

  it("shows a translated original when present", async () => {
    apiMock.signal.mockResolvedValue(signal({ translated: true, language: "fr", originalTitle: "Séisme" }));
    renderWithProviders(<SignalDrawer signalId="sg" onClose={() => {}} />);
    expect(await screen.findByText(/Séisme/)).toBeInTheDocument();
    expect(screen.getByText(/Translated from French/)).toBeInTheDocument();
  });

  it("calls onClose from the drawer close button", async () => {
    apiMock.signal.mockResolvedValue(signal());
    const onClose = vi.fn();
    renderWithProviders(<SignalDrawer signalId="sg" onClose={onClose} />);
    await screen.findByText("Quake hits coast");
    await userEvent.click(screen.getByRole("button", { name: "Close details" }));
    expect(onClose).toHaveBeenCalled();
  });
});
