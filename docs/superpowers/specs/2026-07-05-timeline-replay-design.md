# Live map — Timeline Replay

Date: 2026-07-05
Status: Approved

## Goal

Turn the live map into a DVR: a play/pause scrubber that replays the currently
loaded time window as an animation, so you can watch events appear over time
instead of seeing the whole window at once.

## Decisions

- **Model:** freeze & scrub the loaded window. Entering Replay pauses live
  polling (events freeze), captures `replayEnd = now`, and sweeps a playhead
  from `replayEnd − windowMs` → `replayEnd`. No new fetch, no backend changes.
- **Playback visual:** cumulative + fade. At playhead `t`, show events with
  `lastSeenMs ≤ t`, faded by age relative to `t` (reusing `recencyOpacity`),
  with a ripple for events that just crossed `t`. Ends looking identical to the
  live map.
- **Loop:** off (one sweep, stops at the live-equivalent frame).
- **Replay state:** local (not URL). Window/country/view stay in the URL.

## Design

### Interaction & controls
- **Replay** toggle (icon button in the top bar near the view toggle). Entering
  auto-plays from the window start.
- **Bottom control bar** over the map: `⏵/⏸`, a draggable **scrubber** (Mantine
  Slider across the window), the **playhead timestamp**, a **speed toggle
  (1×/2×/4×)**, and **✕ Go live** to exit and resume polling.
- 1× sweep ≈ 24s regardless of window; speed shortens it. Reaching the end stops
  at the last frame. Dragging the scrubber pauses and seeks.
- Category/subcategory layer filters still apply (render-time). Changing
  **window or country exits replay** (needs a fresh fetch). No breaking toasts
  fire while replaying (polling paused).

### Data / rendering model
- Pausing the poll freezes `markers` state — no separate snapshot. The map shows
  `frameMarkers(shown, t, windowMs)`: events with `lastSeenMs ≤ t`, opacity from
  age relative to `t`, `isNew` for events crossing `t` this tick. Events without
  a timestamp (`lastSeenMs = 0`) are excluded from replay.
- Side-rail pulse/ticker stay on the frozen window (not tied to the playhead in
  this pass — future work).

### Modules & boundaries
- `lib/replay.ts` — pure `frameMarkers(recs, playheadMs, windowMs, prevPlayheadMs)`
  → visible markers with opacity/`isNew`. Fully unit-tested.
- `lib/useReplay.ts` — hook owning `{ playing, playheadMs, speed }`, advancing the
  playhead on an interval; play/pause/seek/setSpeed; clamps and stops at the end.
  Fake-timer tested.
- `components/ReplayBar.tsx` — bottom control bar (dumb; driven by hook state +
  callbacks).
- `pages/LiveDashboard.tsx` — replay on/off, pause the poll when active, render
  `ReplayBar`, feed `frameMarkers(...)` to `LiveMap`.

### Testing / verification
- Unit: `frameMarkers` (playhead filter, fade, just-appeared ripple, no-timestamp
  exclusion) and `useReplay` (advance, clamp/stop at end, pause/seek, speed) with
  fake timers.
- Integration: `LiveDashboard` — entering replay pauses polling and shows fewer
  markers early; scrubbing changes the visible count; Go-live resumes.
- Browser: drive `/live` end-to-end (play, scrub, speed, exit) + GIF/screenshots.
- Frontend coverage gate ≥95%. **No backend changes** (backend stays green).

## Out of scope

Tying the pulse/ticker to the playhead; fetching a denser history beyond the
2000-marker window cap; looping playback.
