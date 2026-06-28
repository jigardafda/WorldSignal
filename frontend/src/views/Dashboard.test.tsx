import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Dashboard } from "./Dashboard";
import { api, type Signal } from "../api";

vi.mock("../api", () => ({ api: { stats: vi.fn(), signals: vi.fn() } }));

const signal = (over: Partial<Signal> = {}): Signal => ({
  id: "sig", title: "Quake", summary: "s", status: "CONFIRMED", severity: "HIGH",
  confidence: 0.8, sourceCount: 3, firstSeenAt: "", lastSeenAt: "",
  tags: [{ code: "DISASTER.EARTHQUAKE", confidence: 0.9 }], sources: [], ...over,
});

afterEach(() => vi.clearAllMocks());

describe("Dashboard", () => {
  it("renders stat cards and latest signals", async () => {
    (api.stats as any).mockResolvedValue({ sources: 2, articles: 4, signals: 1, deliveriesSent: 5, deliveriesPending: 1 });
    (api.signals as any).mockResolvedValue([signal()]);
    render(<Dashboard />);
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    expect(screen.getByText("Sources")).toBeInTheDocument();
    expect(screen.getByText("DISASTER.EARTHQUAKE")).toBeInTheDocument();
  });

  it("shows empty state when no signals", async () => {
    (api.stats as any).mockResolvedValue({ sources: 0, articles: 0, signals: 0, deliveriesSent: 0, deliveriesPending: 0 });
    (api.signals as any).mockResolvedValue([]);
    render(<Dashboard />);
    expect(await screen.findByText(/No signals yet/)).toBeInTheDocument();
  });

  it("shows error when the API is unreachable", async () => {
    (api.stats as any).mockRejectedValue(new Error("down"));
    (api.signals as any).mockResolvedValue([]);
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText(/Cannot reach API: down/)).toBeInTheDocument());
  });
});
