import { ColorSwatch, Group, Stack, Text, UnstyledButton } from "@mantine/core";
import { tickerItems, timeAgo } from "../lib/livePulse";
import { useNow } from "../lib/useNow";

export interface TickerRec {
  id: string;
  title: string;
  color?: string;
  lastSeenMs: number;
  lat: number;
  lng: number;
}

/** Scrolling newest-first feed of live events. Clicking a row invokes `onPick`
 * (which opens the detail drawer and flies the map to the marker). */
export function LiveTicker<T extends TickerRec>({ recs, onPick, max = 40 }: { recs: T[]; onPick: (rec: T) => void; max?: number }) {
  const now = useNow(1000);
  const items = tickerItems(recs, max);

  if (items.length === 0) {
    return <Text size="xs" c="dimmed" data-testid="live-ticker">No recent events.</Text>;
  }
  return (
    <Stack gap={1} data-testid="live-ticker">
      {items.map((r) => (
        <UnstyledButton
          key={r.id}
          onClick={() => onPick(r)}
          data-testid={`ticker-${r.id}`}
          style={{ borderRadius: 4, padding: "3px 4px" }}
          className="ws-ticker-row"
        >
          <Group gap={6} wrap="nowrap">
            <ColorSwatch color={r.color ?? "#868e96"} size={8} />
            <Text size="xs" truncate style={{ flex: 1 }}>{r.title}</Text>
            <Text size="xs" c="dimmed" style={{ flexShrink: 0 }}>{timeAgo(r.lastSeenMs, now)}</Text>
          </Group>
        </UnstyledButton>
      ))}
    </Stack>
  );
}
