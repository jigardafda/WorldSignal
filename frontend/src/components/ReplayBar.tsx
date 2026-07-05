import { ActionIcon, Badge, Group, Paper, Slider, Text } from "@mantine/core";
import { IconPlayerPauseFilled, IconPlayerPlayFilled, IconRotateClockwise, IconX } from "@tabler/icons-react";

function clock(ms: number): string {
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

/** Bottom control bar for timeline replay: play/pause, a scrubber across the
 * window, the current playhead time, a speed cycler, and exit-to-live. Dumb —
 * all state comes from useReplay via props. */
export function ReplayBar({
  playing,
  playheadMs,
  progress,
  speed,
  atEnd,
  onPlayPause,
  onSeek,
  onCycleSpeed,
  onExit,
}: {
  playing: boolean;
  playheadMs: number;
  progress: number; // 0..1
  speed: number;
  atEnd: boolean;
  onPlayPause: () => void;
  onSeek: (progress: number) => void; // 0..1
  onCycleSpeed: () => void;
  onExit: () => void;
}) {
  const PlayIcon = playing ? IconPlayerPauseFilled : atEnd ? IconRotateClockwise : IconPlayerPlayFilled;
  return (
    <Paper
      withBorder
      shadow="md"
      radius="md"
      p="xs"
      style={{ position: "absolute", bottom: 16, left: "50%", transform: "translateX(-50%)", zIndex: 1100, width: "min(680px, calc(100% - 48px))" }}
      data-testid="replay-bar"
    >
      <Group gap="sm" wrap="nowrap">
        <ActionIcon variant="filled" radius="xl" size="lg" onClick={onPlayPause} aria-label={playing ? "Pause" : "Play"} data-testid="replay-playpause">
          <PlayIcon size={18} />
        </ActionIcon>
        <Text size="xs" ff="monospace" c="dimmed" w={70} ta="center" data-testid="replay-time">{clock(playheadMs)}</Text>
        <Slider
          flex={1}
          value={Math.round(progress * 1000) / 10}
          min={0}
          max={100}
          step={0.1}
          label={null}
          onChange={(v) => onSeek(v / 100)}
          data-testid="replay-scrubber"
          aria-label="Replay position"
        />
        <Badge
          variant="light"
          color="blue"
          style={{ cursor: "pointer" }}
          onClick={onCycleSpeed}
          data-testid="replay-speed"
        >
          {speed}×
        </Badge>
        <ActionIcon variant="subtle" color="gray" onClick={onExit} aria-label="Exit replay, go live" data-testid="replay-exit">
          <IconX size={16} />
        </ActionIcon>
      </Group>
    </Paper>
  );
}
