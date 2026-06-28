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

/** Derives a source's polling status from enabled + cooldownUntil. */
export function pollStatusOf(s: { enabled?: boolean; cooldownUntil?: string | null }): "ACTIVE" | "COOLDOWN" | "DISABLED" {
  if (s.enabled === false) return "DISABLED";
  if (s.cooldownUntil && new Date(s.cooldownUntil) > new Date()) return "COOLDOWN";
  return "ACTIVE";
}

const POLL_COLORS: Record<string, string> = { ACTIVE: "green", COOLDOWN: "orange", DISABLED: "gray" };
const POLL_LABELS: Record<string, string> = { ACTIVE: "Polling", COOLDOWN: "Cooldown", DISABLED: "Disabled" };

export function PollBadge({ source }: { source: { enabled?: boolean; cooldownUntil?: string | null } }) {
  const st = pollStatusOf(source);
  return <Badge color={POLL_COLORS[st]} variant={st === "ACTIVE" ? "filled" : "light"}>{POLL_LABELS[st]}</Badge>;
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

const SENTIMENT_COLORS: Record<string, string> = { POSITIVE: "green", NEGATIVE: "red", NEUTRAL: "gray", MIXED: "yellow" };

export function SentimentBadge({ sentiment, size = "sm" }: { sentiment?: string | null; size?: string }) {
  if (!sentiment) return null;
  return <Badge size={size} color={SENTIMENT_COLORS[sentiment] ?? "gray"} variant="light">{sentiment}</Badge>;
}

const INFLUENCE_COLORS: Record<string, string> = { NEGLIGIBLE: "gray", LOW: "blue", MEDIUM: "grape", HIGH: "orange", CRITICAL: "red" };

export function InfluenceBadge({ influence, size = "sm" }: { influence?: string | null; size?: string }) {
  if (!influence) return null;
  return <Badge size={size} color={INFLUENCE_COLORS[influence] ?? "gray"} variant="light">Influence: {influence}</Badge>;
}

/** Minimal shape SignalIntel needs — a subset of api.Signal. */
export interface SignalIntelData {
  region?: string | null; city?: string | null; geoScope?: string | null;
  sentiment?: string | null; influence?: string | null; relevance?: number | null;
  attributes?: { key: string; valueCode: string }[];
}

/** Compact summary of a signal's enrichment, for list rows and cards. */
export function SignalIntel({ signal }: { signal: SignalIntelData }) {
  const loc = [signal.city, signal.region].filter(Boolean).join(", ") || null;
  const industries = (signal.attributes ?? []).filter((a) => a.key === "industry").slice(0, 3);
  const hasAny = loc || signal.sentiment || signal.influence || signal.relevance != null || industries.length > 0;
  if (!hasAny) return null;
  return (
    <Group gap={6} mt={3} wrap="wrap" data-testid="signal-intel">
      {loc && <Text size="xs" c="dimmed">📍 {loc}</Text>}
      <SentimentBadge sentiment={signal.sentiment} size="xs" />
      <InfluenceBadge influence={signal.influence} size="xs" />
      {signal.relevance != null && <Badge size="xs" variant="light" color="blue">Rel {Math.round(signal.relevance * 100)}%</Badge>}
      {industries.map((a) => <Badge key={a.valueCode} size="xs" variant="outline">{a.valueCode}</Badge>)}
    </Group>
  );
}
