import { Group, Paper, Text, ThemeIcon } from "@mantine/core";
import type { ReactNode } from "react";

export function StatCard({ label, value, icon, color = "brand" }: {
  label: string;
  value: ReactNode;
  icon?: ReactNode;
  color?: string;
}) {
  return (
    <Paper withBorder p="md" radius="lg" className="ws-lift" style={{ position: "relative", overflow: "hidden" }}>
      {/* A thin accent rail gives each tile identity without shouting. */}
      <div style={{ position: "absolute", insetInlineStart: 0, top: 0, bottom: 0, width: 4, background: `var(--mantine-color-${color}-6, var(--mantine-color-blue-6))` }} />
      <Group justify="space-between" wrap="nowrap" pl={6}>
        <div>
          <Text size="xs" c="dimmed" tt="uppercase" fw={700} style={{ letterSpacing: "0.04em" }}>{label}</Text>
          <Text fw={800} fz={28} lh={1.1} style={{ fontFamily: "'Space Grotesk Variable', sans-serif" }}>{value ?? "—"}</Text>
        </div>
        {icon && <ThemeIcon variant="light" color={color} size={44} radius="md">{icon}</ThemeIcon>}
      </Group>
    </Paper>
  );
}
