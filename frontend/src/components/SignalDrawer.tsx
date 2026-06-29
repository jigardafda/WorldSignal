import { Badge, Button, Drawer, Group, List, Stack, Text, Title } from "@mantine/core";
import { IconArrowUpRight, IconLanguage } from "@tabler/icons-react";
import { useNavigate } from "react-router-dom";
import { api, type Signal } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, EmptyState } from "./States";
import { ConfidenceBar, InfluenceBadge, SentimentBadge, SeverityBadge, StatusBadge } from "./badges";
import { ExtLink } from "./ExtLink";
import { useCountries, countryDisplay } from "../lib/countries";
import { categoryColor, domainOf } from "../lib/categories";
import { languageName, pct } from "../lib/format";

/** A right-side drawer showing the full details of a signal, opened by clicking a
 * marker on the live map. Fetches the signal on open. */
export function SignalDrawer({ signalId, onClose }: { signalId: string | null; onClose: () => void }) {
  const navigate = useNavigate();
  const { byCode } = useCountries();
  const state = useAsync<Signal | null>(() => (signalId ? api.signal(signalId) : Promise.resolve(null)), [signalId]);

  return (
    <Drawer opened={!!signalId} onClose={onClose} position="right" size="md" title="Signal details" data-testid="signal-drawer" zIndex={2000} closeButtonProps={{ "aria-label": "Close details" }}>
      {signalId && (
        <AsyncBoundary state={state}>
          {(s) =>
            s === null ? (
              <EmptyState message="Signal not found." />
            ) : (
              <Stack gap="sm">
                <Group gap="xs">
                  <SeverityBadge severity={s.severity} />
                  <StatusBadge status={s.status} />
                  {s.eventType && <Badge variant="dot" color={categoryColor(domainOf(s.eventType))}>{s.eventType}</Badge>}
                  {s.translated && (
                    <Badge variant="light" color="indigo" leftSection={<IconLanguage size={12} />}>
                      Translated from {languageName(s.language)}
                    </Badge>
                  )}
                </Group>

                {s.translated && s.originalTitle && (
                  <Text size="sm" c="dimmed" dir="auto"><b>Original:</b> {s.originalTitle}</Text>
                )}
                <Title order={4}>{s.title}</Title>
                <ConfidenceBar value={s.confidence} />

                <Group gap="xs">
                  <SentimentBadge sentiment={s.sentiment} size="sm" />
                  <InfluenceBadge influence={s.influence} size="sm" />
                  {s.relevance != null && <Badge variant="light" color="blue">Relevance {pct(s.relevance)}</Badge>}
                </Group>

                <Text size="sm">{s.summary}</Text>
                {s.whatHappened && (<><Text size="xs" tt="uppercase" c="dimmed" fw={600}>What happened</Text><Text size="sm">{s.whatHappened}</Text></>)}
                {s.whyItMatters && (<><Text size="xs" tt="uppercase" c="dimmed" fw={600}>Why it matters</Text><Text size="sm">{s.whyItMatters}</Text></>)}

                <Stack gap={2} data-testid="drawer-geo">
                  <Text size="xs" tt="uppercase" c="dimmed" fw={600}>Location</Text>
                  <Text size="sm"><b>Country:</b> {countryDisplay(s.country, byCode)}</Text>
                  {s.region && <Text size="sm"><b>Region / State:</b> {s.region}</Text>}
                  {s.city && <Text size="sm"><b>City:</b> {s.city}</Text>}
                  {s.locality && <Text size="sm"><b>Locality:</b> {s.locality}</Text>}
                  {s.geoScope && <Text size="sm"><b>Scope:</b> {s.geoScope}</Text>}
                </Stack>

                {s.tags.length > 0 && (
                  <Group gap={4}>{s.tags.map((t) => <Badge key={t.code} variant="light" size="sm">{t.code}</Badge>)}</Group>
                )}

                <Text size="xs" tt="uppercase" c="dimmed" fw={600}>Sources ({s.sources.length})</Text>
                {s.sources.length === 0 ? (
                  <Text size="sm" c="dimmed">No linked sources.</Text>
                ) : (
                  <List spacing={2} size="sm">
                    {s.sources.slice(0, 8).map((src, i) => (
                      <List.Item key={i}><ExtLink url={src.url}>{src.publisher}</ExtLink></List.Item>
                    ))}
                  </List>
                )}

                <Button
                  variant="light"
                  rightSection={<IconArrowUpRight size={16} />}
                  onClick={() => navigate(`/signals/${s.id}`)}
                  data-testid="drawer-open-full"
                >
                  Open full details
                </Button>
              </Stack>
            )
          }
        </AsyncBoundary>
      )}
    </Drawer>
  );
}
