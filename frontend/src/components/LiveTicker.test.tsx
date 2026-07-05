import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";
import { LiveTicker, type TickerRec } from "./LiveTicker";

const now = Date.now();
const rec = (id: string, secsAgo: number): TickerRec => ({ id, title: id, color: "#e03131", lastSeenMs: now - secsAgo * 1000, lat: 1, lng: 2 });

describe("LiveTicker", () => {
  it("renders newest-first and fires onPick with the clicked record", () => {
    const onPick = vi.fn();
    renderWithProviders(<LiveTicker recs={[rec("old", 300), rec("new", 5), rec("mid", 60)]} onPick={onPick} />);
    const list = screen.getByTestId("live-ticker");
    const ids = within(list).getAllByTestId(/^ticker-/).map((el) => el.getAttribute("data-testid"));
    expect(ids).toEqual(["ticker-new", "ticker-mid", "ticker-old"]);

    fireEvent.click(screen.getByTestId("ticker-new"));
    expect(onPick).toHaveBeenCalledWith(expect.objectContaining({ id: "new", lat: 1, lng: 2 }));
  });

  it("caps the list at `max`", () => {
    const recs = Array.from({ length: 10 }, (_, i) => rec(`s${i}`, i));
    renderWithProviders(<LiveTicker recs={recs} onPick={vi.fn()} max={3} />);
    expect(within(screen.getByTestId("live-ticker")).getAllByTestId(/^ticker-/)).toHaveLength(3);
  });

  it("shows an empty state when there are no events", () => {
    renderWithProviders(<LiveTicker recs={[]} onPick={vi.fn()} />);
    expect(screen.getByTestId("live-ticker")).toHaveTextContent("No recent events.");
  });
});
