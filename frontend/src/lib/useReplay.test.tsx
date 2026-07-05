import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useReplay } from "./useReplay";

const START = 1_000_000_000_000;
const END = START + 60 * 60_000; // 1h window

afterEach(() => vi.useRealTimers());

describe("useReplay", () => {
  it("stays idle at the start when inactive", () => {
    const { result } = renderHook(() => useReplay(START, END, false));
    expect(result.current.playing).toBe(false);
    expect(result.current.playheadMs).toBe(START);
    expect(result.current.progress).toBe(0);
  });

  it("auto-plays from the start and advances the playhead over time", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useReplay(START, END, true));
    expect(result.current.playing).toBe(true);
    expect(result.current.playheadMs).toBe(START);
    act(() => { vi.advanceTimersByTime(1000); }); // 10 ticks
    expect(result.current.playheadMs).toBeGreaterThan(START);
    expect(result.current.progress).toBeGreaterThan(0);
    expect(result.current.prevPlayheadMs).not.toBeNull();
  });

  it("reaches the end, clamps, and stops playing", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useReplay(START, END, true));
    act(() => { vi.advanceTimersByTime(30_000); }); // > 24s base sweep at 1×
    expect(result.current.playheadMs).toBe(END);
    expect(result.current.atEnd).toBe(true);
    expect(result.current.playing).toBe(false);
  });

  it("seekProgress pauses and jumps to a fraction of the window (no ripple)", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useReplay(START, END, true));
    act(() => { result.current.seekProgress(0.5); });
    expect(result.current.playing).toBe(false);
    expect(result.current.playheadMs).toBe(START + 0.5 * (END - START));
    expect(result.current.prevPlayheadMs).toBeNull();
    // clamps out-of-range
    act(() => { result.current.seekProgress(2); });
    expect(result.current.playheadMs).toBe(END);
  });

  it("play() restarts from the beginning when already at the end", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useReplay(START, END, true));
    act(() => { vi.advanceTimersByTime(30_000); });
    expect(result.current.atEnd).toBe(true);
    act(() => { result.current.play(); });
    expect(result.current.playing).toBe(true);
    expect(result.current.playheadMs).toBe(START);
  });

  it("cycles speed 1 → 2 → 4 → 1 and pause() stops advancing", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useReplay(START, END, true));
    expect(result.current.speed).toBe(1);
    act(() => { result.current.cycleSpeed(); });
    expect(result.current.speed).toBe(2);
    act(() => { result.current.cycleSpeed(); });
    expect(result.current.speed).toBe(4);
    act(() => { result.current.cycleSpeed(); });
    expect(result.current.speed).toBe(1);

    act(() => { result.current.pause(); });
    const frozen = result.current.playheadMs;
    act(() => { vi.advanceTimersByTime(2000); });
    expect(result.current.playheadMs).toBe(frozen);
  });
});
