import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Signals } from "./Signals";
import { api, type Signal } from "../api";

vi.mock("../api", () => ({ api: { signals: vi.fn() } }));

const sig: Signal = {
  id: "s1", title: "Quake hits", summary: "A quake.", whyItMatters: "Lots affected",
  status: "CONFIRMED", severity: "HIGH", confidence: 0.8, sourceCount: 2, firstSeenAt: "", lastSeenAt: "",
  tags: [{ code: "DISASTER.EARTHQUAKE", confidence: 0.9 }],
  sources: [
    { publisher: "BBC", url: "https://bbc.example/a", publishedAt: null, relation: "PRIMARY" },
    { publisher: "NoLink", url: null, publishedAt: null, relation: "SUPPORTING" },
  ],
};

afterEach(() => vi.clearAllMocks());

describe("Signals", () => {
  it("lists signals and shows detail on click", async () => {
    (api.signals as any).mockResolvedValue([sig]);
    render(<Signals />);
    const row = await screen.findByText("Quake hits");
    await userEvent.click(row);
    expect(screen.getByText("Why it matters")).toBeInTheDocument();
    expect(screen.getByText("A quake.")).toBeInTheDocument();
    // Source with URL renders a link; without URL renders plain text.
    expect(screen.getByRole("link", { name: "BBC" })).toHaveAttribute("href", "https://bbc.example/a");
    expect(screen.getByText("NoLink")).toBeInTheDocument();
  });

  it("searches on button click and filters by confidence", async () => {
    (api.signals as any).mockResolvedValue([sig]);
    render(<Signals />);
    await screen.findByText("Quake hits");

    await userEvent.type(screen.getByPlaceholderText("Search signals…"), "quake");
    await userEvent.click(screen.getByRole("button", { name: /Search/ }));
    await waitFor(() => expect(api.signals).toHaveBeenCalledWith(expect.objectContaining({ search: "quake" }), 100));

    await userEvent.selectOptions(screen.getByRole("combobox"), "0.7");
    await waitFor(() => expect(api.signals).toHaveBeenCalledWith(expect.objectContaining({ minConfidence: 0.7 }), 100));
  });

  it("submits search on Enter and shows empty state", async () => {
    (api.signals as any).mockResolvedValue([]);
    render(<Signals />);
    await screen.findByText("No matching signals.");
    await userEvent.type(screen.getByPlaceholderText("Search signals…"), "x{Enter}");
    await waitFor(() => expect(api.signals).toHaveBeenCalled());
    expect(screen.getByText(/Select a signal/)).toBeInTheDocument();
  });
});
