import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { api } from "./api";

vi.mock("./api", () => ({
  api: {
    stats: vi.fn().mockResolvedValue({ sources: 0, articles: 0, signals: 0, deliveriesSent: 0, deliveriesPending: 0 }),
    signals: vi.fn().mockResolvedValue([]),
    sources: vi.fn().mockResolvedValue([]),
    taxonomy: vi.fn().mockResolvedValue([]),
    createSource: vi.fn(),
    setSourceEnabled: vi.fn(),
    fetchSource: vi.fn(),
  },
}));

afterEach(() => vi.clearAllMocks());

describe("App", () => {
  it("defaults to the dashboard and switches tabs", async () => {
    render(<App />);
    expect(screen.getByRole("heading", { name: "Dashboard" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Signal Explorer" }));
    expect(screen.getByRole("heading", { name: "Signal Explorer" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Sources" }));
    expect(screen.getByRole("heading", { name: "Sources" })).toBeInTheDocument();
    expect(api.sources).toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "Taxonomy" }));
    expect(screen.getByRole("heading", { name: "WorldSignal Taxonomy" })).toBeInTheDocument();
  });
});
