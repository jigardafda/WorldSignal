import { Group, Paper, Text, ThemeIcon } from "@mantine/core";
import type { ReactNode } from "react";

export function StatCard({ label, value, icon, color = "blue" }: {
  label: string;
  value: ReactNode;
  icon?: ReactNode;
  color?: string;
}) {
  return (
    <Paper withBorder p="md" radius="md">
      <Group justify="space-between" wrap="nowrap">
        <div>
          <Text size="xs" c="dimmed" tt="uppercase" fw={700}>{label}</Text>
          <Text fw={700} size="xl">{value ?? "—"}</Text>
        </div>
        {icon && <ThemeIcon variant="light" color={color} size={38} radius="md">{icon}</ThemeIcon>}
      </Group>
    </Paper>
  );
}
