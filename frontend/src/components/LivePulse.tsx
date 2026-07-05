import { ColorSwatch, Group, Stack, Text } from "@mantine/core";
import { IconArrowRight, IconTrendingUp } from "@tabler/icons-react";
import type { Country } from "../lib/api";
import { categoryColor, categoryLabel } from "../lib/categories";
import { topMovers, velocity, type Mover, type PulseRec } from "../lib/livePulse";
import { useNow } from "../lib/useNow";

function moverLabel(m: Mover): string {
  return m.older === 0 ? "new" : `${m.ratio.toFixed(1)}×`;
}

function MoverRow({ swatch, label, m, testid }: { swatch: React.ReactNode; label: string; m: Mover; testid: string }) {
  const rising = m.ratio > 1;
  return (
    <Group gap={6} wrap="nowrap" justify="space-between" data-testid={testid}>
      <Group gap={6} wrap="nowrap" style={{ minWidth: 0 }}>
        {swatch}
        <Text size="xs" truncate>{label}</Text>
      </Group>
      <Group gap={4} wrap="nowrap">
        {rising ? <IconTrendingUp size={12} color="var(--mantine-color-teal-6)" /> : <IconArrowRight size={12} color="var(--mantine-color-gray-5)" />}
        <Text size="xs" c={rising ? "teal" : "dimmed"} fw={600}>{moverLabel(m)}</Text>
        <Text size="xs" c="dimmed">{m.recent}</Text>
      </Group>
    </Group>
  );
}

/** Live "pulse" panel: current events/min velocity, plus the categories and
 * countries surging in the recent half of the window vs the older half. */
export function LivePulse({ recs, windowMs, byCode }: { recs: PulseRec[]; windowMs: number; byCode: Record<string, Country> }) {
  const now = useNow(1000);
  const v = velocity(recs, now);
  const catMovers = topMovers(recs, (r) => r.category, now, windowMs, 3);
  const countryMovers = topMovers(recs, (r) => r.country, now, windowMs, 3);

  return (
    <Stack gap={8} data-testid="live-pulse">
      <Group gap={6} align="baseline" data-testid="pulse-velocity">
        <Text size="xl" fw={800} lh={1}>{v.perMin}</Text>
        <Text size="xs" c="dimmed">events/min</Text>
      </Group>

      <div>
        <Text size="xs" tt="uppercase" c="dimmed" fw={700} mb={2}>Top movers</Text>
        {catMovers.length === 0 ? (
          <Text size="xs" c="dimmed">Gathering trend…</Text>
        ) : (
          <Stack gap={3}>
            {catMovers.map((m) => (
              <MoverRow key={m.key} m={m} testid={`mover-cat-${m.key}`}
                swatch={<ColorSwatch color={categoryColor(m.key)} size={9} />}
                label={categoryLabel(m.key)} />
            ))}
          </Stack>
        )}
      </div>

      {countryMovers.length > 0 && (
        <div>
          <Text size="xs" tt="uppercase" c="dimmed" fw={700} mb={2}>Hotspots</Text>
          <Stack gap={3}>
            {countryMovers.map((m) => (
              <MoverRow key={m.key} m={m} testid={`mover-country-${m.key}`}
                swatch={<Text size="xs">{byCode[m.key]?.flag ?? "🏳️"}</Text>}
                label={byCode[m.key]?.name ?? m.key} />
            ))}
          </Stack>
        </div>
      )}
    </Stack>
  );
}
