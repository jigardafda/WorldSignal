import { ActionIcon, Anchor, Group, Paper, Text, ThemeIcon, Tooltip } from "@mantine/core";
import { IconAlertTriangle, IconBell, IconBellOff, IconX } from "@tabler/icons-react";

export interface BreakingAlert {
  key: number;
  count: number;
  title: string;
  firstId: string;
  expiresAt: number;
}

interface Props {
  alerts: BreakingAlert[];
  paused: boolean;
  onTogglePause: () => void;
  onOpen: (id: string) => void;
  onDismiss: (key: number) => void;
}

/** Compact, bottom-right stack of breaking-signal alerts (at most two visible at
 * once) with a bell toggle to mute new ones. Replaces the app-wide Mantine toast
 * so the live map owns the position, density, and muting of its own alerts. */
export function BreakingAlerts({ alerts, paused, onTogglePause, onOpen, onDismiss }: Props) {
  const visible = alerts.slice(0, 2);
  // Nothing to show and not muted → render nothing (keeps the map clean).
  if (!paused && visible.length === 0) return null;

  return (
    <div
      style={{
        position: "absolute", bottom: 12, right: 12, zIndex: 1100, width: 300,
        display: "flex", flexDirection: "column", gap: 8, alignItems: "flex-end",
      }}
      data-testid="breaking-alerts"
    >
      {visible.map((a) => (
        <Paper key={a.key} withBorder shadow="md" radius="md" p={8} style={{ width: "100%" }} data-testid="breaking-alert">
          <Group gap={8} wrap="nowrap" align="flex-start">
            <ThemeIcon color="red" radius="xl" size="sm" variant="filled" mt={1}>
              <IconAlertTriangle size={13} />
            </ThemeIcon>
            <div style={{ flex: 1, minWidth: 0 }}>
              <Text size="xs" fw={700} lh={1.2}>
                {a.count === 1 ? "Breaking signal" : `${a.count} breaking signals`}
              </Text>
              <Anchor size="xs" lineClamp={2} style={{ cursor: "pointer" }} onClick={() => onOpen(a.firstId)}>
                {a.title}
              </Anchor>
            </div>
            <ActionIcon variant="subtle" color="gray" size="sm" aria-label="Dismiss alert" data-testid="breaking-dismiss" onClick={() => onDismiss(a.key)}>
              <IconX size={14} />
            </ActionIcon>
          </Group>
        </Paper>
      ))}
      <Group gap={6} wrap="nowrap">
        {paused && visible.length === 0 && (
          <Text size="xs" c="dimmed" data-testid="breaking-paused-note">Breaking alerts paused</Text>
        )}
        <Tooltip label={paused ? "Resume breaking alerts" : "Pause breaking alerts"} position="left" withArrow>
          <ActionIcon
            variant={paused ? "filled" : "default"}
            color={paused ? "red" : "gray"}
            size="md"
            radius="xl"
            aria-label={paused ? "Resume breaking alerts" : "Pause breaking alerts"}
            data-testid="breaking-pause"
            onClick={onTogglePause}
          >
            {paused ? <IconBellOff size={16} /> : <IconBell size={16} />}
          </ActionIcon>
        </Tooltip>
      </Group>
    </div>
  );
}
