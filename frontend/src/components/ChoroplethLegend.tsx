import { Group, Paper, Text } from "@mantine/core";
import { legendFor, type Metric } from "../lib/choropleth";

const TITLE: Record<Metric, string> = { count: "Signals per country", severity: "% high-severity", sentiment: "Net sentiment" };

/** The color-scale legend for the choropleth: a gradient bar with end labels. */
export function ChoroplethLegend({ metric, max }: { metric: Metric; max: number }) {
  const { stops, min, max: maxLabel } = legendFor(metric, max);
  const gradient = `linear-gradient(to right, ${stops.join(", ")})`;
  return (
    <Paper
      withBorder
      shadow="md"
      radius="md"
      p="xs"
      style={{ position: "absolute", bottom: 16, left: 16, zIndex: 1000, width: 200 }}
      data-testid="choropleth-legend"
    >
      <Text size="xs" fw={700} mb={4}>{TITLE[metric]}</Text>
      <div style={{ height: 10, borderRadius: 4, background: gradient }} />
      <Group justify="space-between" mt={2} gap={0}>
        <Text size="xs" c="dimmed">{min}</Text>
        <Text size="xs" c="dimmed">{maxLabel}</Text>
      </Group>
    </Paper>
  );
}
