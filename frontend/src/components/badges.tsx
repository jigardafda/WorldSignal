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

const VALIDATION_COLORS: Record<string, string> = { VALID: "green", INVALID: "red", PENDING: "yellow" };

export function ValidationBadge({ status }: { status?: string }) {
  if (!status) return <Text size="sm" c="dimmed">—</Text>;
  return <Badge color={VALIDATION_COLORS[status] ?? "gray"} variant="light">{status}</Badge>;
}

export function HealthBadge({ score }: { score?: number | null }) {
  if (score == null) return <Text size="sm" c="dimmed">—</Text>;
  const color = score >= 85 ? "green" : score >= 60 ? "yellow" : score >= 40 ? "orange" : "red";
  return <Badge color={color} variant="light">{score}</Badge>;
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
