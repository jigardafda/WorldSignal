import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useNow } from "./useNow";

afterEach(() => vi.useRealTimers());

describe("useNow", () => {
  it("returns a timestamp that advances on each interval tick", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useNow(1000));
    const first = result.current;
    expect(typeof first).toBe("number");
    act(() => { vi.advanceTimersByTime(1000); });
    expect(result.current).toBeGreaterThanOrEqual(first);
  });

  it("clears its interval on unmount", () => {
    vi.useFakeTimers();
    const clear = vi.spyOn(globalThis, "clearInterval");
    const { unmount } = renderHook(() => useNow(500));
    unmount();
    expect(clear).toHaveBeenCalled();
  });
});
