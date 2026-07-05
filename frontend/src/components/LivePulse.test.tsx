import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../test/utils";
import { LivePulse } from "./LivePulse";
import type { PulseRec } from "../lib/livePulse";
import type { Country } from "../lib/api";

const now = Date.now();
const rec = (id: string, category: string, country: string, secsAgo: number): PulseRec => ({
  id, title: id, category, country, lat: 0, lng: 0, lastSeenMs: now - secsAgo * 1000,
});
const byCode: Record<string, Country> = {
  JP: { code: "JP", name: "Japan", flag: "🇯🇵", currency: "JPY", capital: "Tokyo", capitalLat: 35, capitalLng: 139 },
};

describe("LivePulse", () => {
  it("shows events/min velocity and surging category + country movers", () => {
    const recs = [
      rec("a", "CONFLICT", "JP", 5),
      rec("b", "CONFLICT", "JP", 20),
      rec("c", "CONFLICT", "JP", 40),
    ];
    renderWithProviders(<LivePulse recs={recs} windowMs={60 * 60_000} byCode={byCode} />);
    expect(screen.getByTestId("pulse-velocity")).toHaveTextContent("3"); // 3 events in last 60s
    // CONFLICT surged from nothing in the older half ⇒ "new"
    expect(screen.getByTestId("mover-cat-CONFLICT")).toHaveTextContent(/new/);
    // Country hotspot renders with its flag/name.
    expect(screen.getByTestId("mover-country-JP")).toHaveTextContent("Japan");
  });

  it("shows a placeholder when there is nothing recent to trend", () => {
    renderWithProviders(<LivePulse recs={[]} windowMs={60 * 60_000} byCode={byCode} />);
    expect(screen.getByTestId("pulse-velocity")).toHaveTextContent("0");
    expect(screen.getByText(/Gathering trend/)).toBeInTheDocument();
  });
});
