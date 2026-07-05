import { useCallback, useEffect, useState } from "react";

// A 1× sweep of the whole window takes BASE_MS; speeds shorten it. The playhead
// advances every TICK_MS by (window / ticks-per-sweep) × speed.
const BASE_MS = 24_000;
const TICK_MS = 100;
export const SPEEDS = [1, 2, 4] as const;

export interface ReplayControls {
  playing: boolean;
  playheadMs: number;
  prevPlayheadMs: number | null; // previous frame's playhead (for ripples); null after a seek
  speed: number;
  progress: number; // 0..1 across the window
  atEnd: boolean;
  play(): void;
  pause(): void;
  seekProgress(p: number): void;
  cycleSpeed(): void;
}

/**
 * Drives the timeline-replay playhead across `[startMs, endMs]`. Owns
 * play/pause, speed, and seeking; auto-restarts and auto-plays whenever a replay
 * session begins (`active` flips true) or its bounds change, and stops at the
 * end of the sweep. Pure state + timers, so it's fake-timer testable.
 */
export function useReplay(startMs: number, endMs: number, active: boolean): ReplayControls {
  const windowMs = Math.max(1, endMs - startMs);
  const [frame, setFrame] = useState<{ playhead: number; prev: number | null }>({ playhead: startMs, prev: null });
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);

  // Begin (or rebound) a replay session: rewind to the start and auto-play.
  useEffect(() => {
    if (active) {
      setFrame({ playhead: startMs, prev: null });
      setSpeed(1);
      setPlaying(true);
    } else {
      setPlaying(false);
    }
  }, [active, startMs, endMs]);

  // Advance the playhead while playing.
  useEffect(() => {
    if (!active || !playing) return;
    const step = (windowMs / (BASE_MS / TICK_MS)) * speed;
    const t = setInterval(() => {
      setFrame((f) => ({ prev: f.playhead, playhead: Math.min(endMs, f.playhead + step) }));
    }, TICK_MS);
    return () => clearInterval(t);
  }, [active, playing, speed, windowMs, endMs]);

  // Stop when the sweep reaches the end.
  useEffect(() => {
    if (playing && frame.playhead >= endMs) setPlaying(false);
  }, [playing, frame.playhead, endMs]);

  const play = useCallback(() => {
    setFrame((f) => (f.playhead >= endMs ? { playhead: startMs, prev: null } : { ...f, prev: null }));
    setPlaying(true);
  }, [startMs, endMs]);
  const pause = useCallback(() => setPlaying(false), []);
  const seekProgress = useCallback(
    (p: number) => {
      const clamped = Math.min(1, Math.max(0, p));
      setPlaying(false);
      setFrame({ playhead: startMs + clamped * windowMs, prev: null });
    },
    [startMs, windowMs],
  );
  const cycleSpeed = useCallback(() => setSpeed((s) => SPEEDS[(SPEEDS.indexOf(s as (typeof SPEEDS)[number]) + 1) % SPEEDS.length]), []);

  return {
    playing,
    playheadMs: frame.playhead,
    prevPlayheadMs: frame.prev,
    speed,
    progress: (frame.playhead - startMs) / windowMs,
    atEnd: frame.playhead >= endMs,
    play,
    pause,
    seekProgress,
    cycleSpeed,
  };
}
