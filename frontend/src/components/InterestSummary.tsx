import { Anchor, Badge, Group, Paper, Text } from "@mantine/core";
import { IconAdjustments } from "@tabler/icons-react";
import { humanizeReason, reasonColor } from "../lib/relevanceUi";

/** A one-line summary of what the active profile ranks by, shown above the feed
 * so the ordering makes sense. Empty when the profile has no interests yet. */
export function InterestSummary({
  interests,
  onEdit,
}: {
  interests: Record<string, number>;
  onEdit: () => void;
}) {
  const rows = Object.entries(interests).sort((a, b) => b[1] - a[1]);

  return (
    <Paper withBorder radius="md" py="xs" px="md" data-testid="interest-summary">
      <Group justify="space-between" wrap="nowrap" gap="md">
        <Group gap={7} wrap="wrap" style={{ minWidth: 0 }}>
          <Text size="xs" c="dimmed" fw={600} tt="uppercase" style={{ letterSpacing: "0.05em", flex: "none" }}>
            Ranking by
          </Text>
          {rows.length === 0 ? (
            <Text size="sm" c="dimmed">
              Nothing yet — this feed isn't personalized. Add interests to rank it.
            </Text>
          ) : (
            rows.map(([key, w]) => {
              const chip = humanizeReason(key);
              return (
                <Badge
                  key={key}
                  variant="light"
                  color={reasonColor(chip.kind)}
                  radius="sm"
                  rightSection={
                    <Text span fz={10} fw={700} style={{ fontVariantNumeric: "tabular-nums" }}>
                      ×{w}
                    </Text>
                  }
                >
                  {chip.label}
                </Badge>
              );
            })
          )}
        </Group>
        <Anchor component="button" type="button" onClick={onEdit} size="sm" fw={500} style={{ flex: "none" }}>
          <Group gap={4} wrap="nowrap">
            <IconAdjustments size={14} /> Edit
          </Group>
        </Anchor>
      </Group>
    </Paper>
  );
}
