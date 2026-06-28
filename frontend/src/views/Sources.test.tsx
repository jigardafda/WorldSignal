import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Sources } from "./Sources";
import { api, type Source } from "../api";

vi.mock("../api", () => ({
  api: { sources: vi.fn(), createSource: vi.fn(), setSourceEnabled: vi.fn(), fetchSource: vi.fn() },
}));

const src: Source = {
  id: "s1", name: "BBC", type: "RSS", url: "https://bbc.example/feed", country: "GB",
  priority: 1, credibility: 0.9, enabled: true, lastSuccessAt: "2026-01-02T00:00:00.000Z", failureCount: 0,
};

afterEach(() => vi.clearAllMocks());

describe("Sources", () => {
  it("renders the sources table", async () => {
    (api.sources as any).mockResolvedValue([src]);
    render(<Sources />);
    expect(await screen.findByText("BBC")).toBeInTheDocument();
    expect(screen.getByText("90%")).toBeInTheDocument();
    expect(screen.getByText("P1")).toBeInTheDocument();
  });

  it("adds a source via the form", async () => {
    (api.sources as any).mockResolvedValue([]);
    (api.createSource as any).mockResolvedValue({ id: "n", name: "New" });
    render(<Sources />);
    await waitFor(() => expect(api.sources).toHaveBeenCalled());

    await userEvent.type(screen.getByPlaceholderText("Name"), "New Feed");
    await userEvent.type(screen.getByPlaceholderText("RSS/Atom URL"), "https://new.example/feed");
    await userEvent.type(screen.getByPlaceholderText("Country (e.g. US)"), "US");
    await userEvent.click(screen.getByRole("button", { name: "Add source" }));

    await waitFor(() => expect(api.createSource).toHaveBeenCalledWith(
      expect.objectContaining({ name: "New Feed", url: "https://new.example/feed", country: "US", priority: 2 }),
    ));
    expect(await screen.findByText(/Source added/)).toBeInTheDocument();
  });

  it("surfaces an error message when add fails", async () => {
    (api.sources as any).mockResolvedValue([]);
    (api.createSource as any).mockRejectedValue(new Error("dup"));
    render(<Sources />);
    await userEvent.type(screen.getByPlaceholderText("Name"), "X");
    await userEvent.type(screen.getByPlaceholderText("RSS/Atom URL"), "u");
    await userEvent.click(screen.getByRole("button", { name: "Add source" }));
    expect(await screen.findByText("dup")).toBeInTheDocument();
  });

  it("fetches and toggles a source", async () => {
    (api.sources as any).mockResolvedValue([src]);
    (api.fetchSource as any).mockResolvedValue({ queued: true });
    (api.setSourceEnabled as any).mockResolvedValue({ id: "s1", enabled: false });
    render(<Sources />);
    await screen.findByText("BBC");

    await userEvent.click(screen.getByRole("button", { name: "Fetch" }));
    await waitFor(() => expect(api.fetchSource).toHaveBeenCalledWith("s1"));
    expect(await screen.findByText("Queued BBC")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(api.setSourceEnabled).toHaveBeenCalledWith("s1", false));
  });

  it("shows 'never' and disabled styling for unfetched/disabled sources", async () => {
    (api.sources as any).mockResolvedValue([{ ...src, enabled: false, lastSuccessAt: null }]);
    render(<Sources />);
    await screen.findByText("BBC");
    expect(screen.getByText("never")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Enable" })).toBeInTheDocument();
  });
});
