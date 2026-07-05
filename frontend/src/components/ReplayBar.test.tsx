import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";
import { ReplayBar } from "./ReplayBar";

const base = {
  playing: false,
  playheadMs: new Date("2026-07-05T09:30:00").getTime(),
  progress: 0.25,
  speed: 1,
  atEnd: false,
  onPlayPause: vi.fn(),
  onSeek: vi.fn(),
  onCycleSpeed: vi.fn(),
  onExit: vi.fn(),
};

describe("ReplayBar", () => {
  it("shows the playhead time and a Play control when paused", () => {
    renderWithProviders(<ReplayBar {...base} />);
    expect(screen.getByTestId("replay-time")).toHaveTextContent(/09:30/);
    expect(screen.getByTestId("replay-playpause")).toHaveAttribute("aria-label", "Play");
  });

  it("shows a Pause control when playing", () => {
    renderWithProviders(<ReplayBar {...base} playing />);
    expect(screen.getByTestId("replay-playpause")).toHaveAttribute("aria-label", "Pause");
  });

  it("wires play/pause, speed and exit callbacks", () => {
    const onPlayPause = vi.fn(), onCycleSpeed = vi.fn(), onExit = vi.fn();
    renderWithProviders(<ReplayBar {...base} speed={2} onPlayPause={onPlayPause} onCycleSpeed={onCycleSpeed} onExit={onExit} />);
    fireEvent.click(screen.getByTestId("replay-playpause"));
    expect(onPlayPause).toHaveBeenCalled();
    expect(screen.getByTestId("replay-speed")).toHaveTextContent("2×");
    fireEvent.click(screen.getByTestId("replay-speed"));
    expect(onCycleSpeed).toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("replay-exit"));
    expect(onExit).toHaveBeenCalled();
  });

  it("emits a 0..1 progress when the scrubber is moved", () => {
    const onSeek = vi.fn();
    renderWithProviders(<ReplayBar {...base} progress={0.25} onSeek={onSeek} />);
    const thumb = screen.getByRole("slider");
    thumb.focus();
    fireEvent.keyDown(thumb, { key: "ArrowRight" }); // Mantine slider → onChange
    expect(onSeek).toHaveBeenCalled();
    const arg = onSeek.mock.calls.at(-1)![0];
    expect(arg).toBeGreaterThanOrEqual(0);
    expect(arg).toBeLessThanOrEqual(1);
  });
});
