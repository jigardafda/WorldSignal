import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";
import { BreakingAlerts, type BreakingAlert } from "./BreakingAlerts";

const mk = (over: Partial<BreakingAlert> = {}): BreakingAlert => ({
  key: 1, count: 1, title: "Major quake", firstId: "s1", expiresAt: Date.now() + 8000, ...over,
});

describe("BreakingAlerts", () => {
  it("renders nothing when there are no alerts and it is not paused", () => {
    const { container } = renderWithProviders(
      <BreakingAlerts alerts={[]} paused={false} onTogglePause={vi.fn()} onOpen={vi.fn()} onDismiss={vi.fn()} />,
    );
    expect(container.querySelector('[data-testid="breaking-alerts"]')).toBeNull();
  });

  it("shows at most two cards even when handed more", () => {
    const alerts = [mk({ key: 3 }), mk({ key: 2 }), mk({ key: 1 })];
    renderWithProviders(
      <BreakingAlerts alerts={alerts} paused={false} onTogglePause={vi.fn()} onOpen={vi.fn()} onDismiss={vi.fn()} />,
    );
    expect(screen.getAllByTestId("breaking-alert")).toHaveLength(2);
  });

  it("pluralizes the header by count", () => {
    renderWithProviders(
      <BreakingAlerts alerts={[mk({ count: 5 })]} paused={false} onTogglePause={vi.fn()} onOpen={vi.fn()} onDismiss={vi.fn()} />,
    );
    expect(screen.getByText("5 breaking signals")).toBeInTheDocument();
  });

  it("opens the first signal and dismisses by key", () => {
    const onOpen = vi.fn();
    const onDismiss = vi.fn();
    renderWithProviders(
      <BreakingAlerts alerts={[mk({ key: 7, firstId: "sig-7" })]} paused={false} onTogglePause={vi.fn()} onOpen={onOpen} onDismiss={onDismiss} />,
    );
    fireEvent.click(screen.getByText("Major quake"));
    expect(onOpen).toHaveBeenCalledWith("sig-7");
    fireEvent.click(screen.getByTestId("breaking-dismiss"));
    expect(onDismiss).toHaveBeenCalledWith(7);
  });

  it("exposes a pause toggle, and when paused shows the muted note", () => {
    const onTogglePause = vi.fn();
    renderWithProviders(
      <BreakingAlerts alerts={[]} paused onTogglePause={onTogglePause} onOpen={vi.fn()} onDismiss={vi.fn()} />,
    );
    expect(screen.getByTestId("breaking-paused-note")).toBeInTheDocument();
    const toggle = screen.getByTestId("breaking-pause");
    expect(toggle).toHaveAttribute("aria-label", "Resume breaking alerts");
    fireEvent.click(toggle);
    expect(onTogglePause).toHaveBeenCalled();
  });
});
