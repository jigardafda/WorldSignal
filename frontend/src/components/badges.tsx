import { Badge, Group, Progress, Text } from "@mantine/core";

const SEVERITY_COLORS: Record<string, string> = {
  LOW: "gray", MEDIUM: "yellow", HIGH: "orange", CRITICAL: "red",
};
const STATUS_COLORS: Record<string, string> = {
  UNVERIFIED: "gray", DEVELOPING: "blue", CONFIRMED: "green", DISPUTED: "orange",
  CORRECTED: "violet", RETRACTED: "red", RESOLVED: "teal",
  ACTIVE: "green", SUSPENDED: "red", DELETED: "gray",
  PENDING: "yellow", SENT: "green", FAILED: "red", RETRYING: "orange", DEAD_LETTERED: "dark",
  PARSED: "green", DUPLICATE: "gray",
  created: "blue", active: "yellow", completed: "green", failed: "red",
  ADMIN: "red", EDITOR: "blue", VIEWER: "gray",
};

export function SeverityBadge({ severity }: { severity: string }) {
  return <Badge color={SEVERITY_COLORS[severity] ?? "gray"} variant="filled">{severity}</Badge>;
}

export function StatusBadge({ status }: { status: string }) {
  return <Badge color={STATUS_COLORS[status] ?? "gray"} variant="light">{status}</Badge>;
}

export function ConfidenceBar({ value }: { value: number }) {
  const v = Math.round(value * 100);
  return (
    <Group gap="xs" wrap="nowrap" w={120}>
      <Progress value={v} w={70} size="sm" color={v >= 70 ? "green" : v >= 50 ? "yellow" : "gray"} />
      <Text size="xs" c="dimmed">{v}%</Text>
    </Group>
  );
}
