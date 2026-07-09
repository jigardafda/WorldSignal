import { ActionIcon, Anchor, Badge, Group, Paper, RingProgress, Stack, Text, Tooltip } from "@mantine/core";
import { IconExternalLink, IconThumbDown, IconThumbUp } from "@tabler/icons-react";
import { Link } from "react-router-dom";
import type { FeedItem, FeedbackAction } from "../lib/api";
import { categoryColor, categoryLabel, domainOf } from "../lib/categories";
import { formatAge, humanizeReason, reasonColor, scoreBand, scorePct } from "../lib/relevanceUi";
import { SentimentBadge, InfluenceBadge, SeverityBadge } from "./badges";

/** One ranked signal: a relevance ring, the story, why it ranked for this
 * profile, and feedback controls that teach the ranking. */
export function SignalFeedCard({
  item,
  vote,
  onFeedback,
}: {
  item: FeedItem;
  vote: FeedbackAction | null;
  onFeedback: (action: FeedbackAction) => void;
}) {
  const band = scoreBand(item.score);
  const dom = domainOf(item.eventType);

  return (
    <Paper withBorder radius="md" p="md" data-testid="feed-card">
      <Group align="flex-start" wrap="nowrap" gap="lg">
        <Stack gap={4} align="center" style={{ flex: "none" }}>
          <Tooltip label={`Relevance ${item.score.toFixed(1)} / 10 — ${band.label}`} withArrow>
            <RingProgress
              size={64}
              thickness={5}
              roundCaps
              sections={[{ value: scorePct(item.score), color: band.color }]}
              label={
                <Text ta="center" fw={700} size="sm" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {item.score.toFixed(1)}
                </Text>
              }
              aria-label={`Relevance score ${item.score.toFixed(1)} out of 10`}
            />
          </Tooltip>
          <Text size="9px" c="dimmed" tt="uppercase" fw={600} style={{ letterSpacing: "0.06em" }}>
            {band.label}
          </Text>
        </Stack>

        <Stack gap={8} style={{ minWidth: 0, flex: 1 }}>
          <Anchor component={Link} to={`/signals/${item.id}`} fw={600} fz="md" lh={1.3} lineClamp={2}>
            {item.title}
          </Anchor>

          <Group gap={6} wrap="wrap">
            {dom && (
              <Badge variant="light" color={categoryColor(dom)} radius="sm" size="sm">
                {categoryLabel(item.eventType) || item.eventType}
              </Badge>
            )}
            {item.country && <Badge variant="default" radius="sm" size="sm">{item.country}</Badge>}
            {item.severity && <SeverityBadge severity={item.severity} />}
            {item.sentiment && <SentimentBadge sentiment={item.sentiment} />}
            {item.influence && <InfluenceBadge influence={item.influence} />}
            <Text size="xs" c="dimmed" style={{ fontVariantNumeric: "tabular-nums" }}>
              · {formatAge(item.ageHours)}
            </Text>
          </Group>

          {item.reasons.length > 0 && (
            <Group gap={6} wrap="wrap" align="center">
              <Text size="xs" c="dimmed" fw={600}>
                Why this ranked:
              </Text>
              {item.reasons.map((r) => {
                const chip = humanizeReason(r);
                return (
                  <Badge key={r} variant="dot" color={reasonColor(chip.kind)} radius="sm" size="sm">
                    {chip.label}
                  </Badge>
                );
              })}
            </Group>
          )}

          <Group gap={6} mt={2}>
            <Tooltip label="More like this" withArrow>
              <ActionIcon
                variant={vote === "UP" ? "filled" : "subtle"}
                color="teal"
                aria-label="More like this"
                aria-pressed={vote === "UP"}
                onClick={() => onFeedback("UP")}
              >
                <IconThumbUp size={17} />
              </ActionIcon>
            </Tooltip>
            <Tooltip label="Less like this" withArrow>
              <ActionIcon
                variant={vote === "DOWN" ? "filled" : "subtle"}
                color="red"
                aria-label="Less like this"
                aria-pressed={vote === "DOWN"}
                onClick={() => onFeedback("DOWN")}
              >
                <IconThumbDown size={17} />
              </ActionIcon>
            </Tooltip>
            <Anchor
              component={Link}
              to={`/signals/${item.id}`}
              ml="auto"
              fz="sm"
              fw={500}
              onClick={() => onFeedback("OPEN")}
            >
              <Group gap={4} wrap="nowrap">
                Open <IconExternalLink size={14} />
              </Group>
            </Anchor>
          </Group>
        </Stack>
      </Group>
    </Paper>
  );
}
