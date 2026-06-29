import { Badge, Button, Card, Grid, Group, List, Paper, Stack, Text, Title } from "@mantine/core";
import { ExtLink } from "../components/ExtLink";
import { IconArrowLeft } from "@tabler/icons-react";
import { useNavigate, useParams } from "react-router-dom";
import { api, type Signal } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { IconLanguage } from "@tabler/icons-react";
import { ConfidenceBar, SeverityBadge, StatusBadge } from "../components/badges";
import { useCountries, countryDisplay } from "../lib/countries";
import { fmtDate, languageName } from "../lib/format";

export function SignalDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { byCode } = useCountries();
  const state = useAsync<Signal | null>(() => api.signal(id), [id]);

  return (
    <>
      <PageHeader
        title="Signal"
        actions={<Button variant="default" leftSection={<IconArrowLeft size={16} />} onClick={() => navigate("/signals")}>Back</Button>}
      />
      <AsyncBoundary state={state}>
        {(s) =>
          s === null ? (
            <EmptyState message="Signal not found." />
          ) : (
            (() => {
            const attrs = s.attributes ?? [];
            const industries = attrs.filter((a) => a.key === "industry");
            const entities = attrs.filter((a) => a.key === "entity");
            const geoBits = [s.locality, s.city, s.region].filter(Boolean).join(", ");
            const pct = (n?: number | null) => (n == null ? null : `${Math.round(n * 100)}%`);
            return (
            <Grid>
              <Grid.Col span={{ base: 12, md: 8 }}>
                <Paper withBorder p="lg" radius="md">
                  <Group mb="sm">
                    <SeverityBadge severity={s.severity} />
                    <StatusBadge status={s.status} />
                    {s.eventType && <Badge variant="outline">{s.eventType}</Badge>}
                    {s.translated && (
                      <Badge variant="light" color="indigo" leftSection={<IconLanguage size={12} />} data-testid="signal-translated">
                        Translated from {languageName(s.language)}
                      </Badge>
                    )}
                  </Group>
                  <Title order={3}>{s.title}</Title>
                  {s.translated && (
                    <Text size="xs" c="dimmed" mt={2}>Auto-translated to English from {languageName(s.language)} during enrichment.</Text>
                  )}
                  <ConfidenceBar value={s.confidence} />
                  <Text mt="md">{s.summary}</Text>
                  {s.whatHappened && (<><Title order={5} mt="md">What happened</Title><Text>{s.whatHappened}</Text></>)}
                  {s.whyItMatters && (<><Title order={5} mt="md">Why it matters</Title><Text>{s.whyItMatters}</Text></>)}
                  <Title order={5} mt="lg">Sources ({s.sources.length})</Title>
                  {s.sources.length === 0 ? (
                    <Text c="dimmed" size="sm">No linked sources.</Text>
                  ) : (
                    <List spacing="xs" mt="xs">
                      {s.sources.map((src, i) => (
                        <List.Item key={i}>
                          <ExtLink url={src.url}>{src.publisher}</ExtLink>
                          {src.publishedAt && <Text span c="dimmed" size="xs"> · {fmtDate(src.publishedAt)}</Text>}
                        </List.Item>
                      ))}
                    </List>
                  )}
                </Paper>
              </Grid.Col>
              <Grid.Col span={{ base: 12, md: 4 }}>
                <Stack>
                  <Card withBorder radius="md">
                    <Title order={5} mb="xs">Details</Title>
                    <Stack gap={4}>
                      <Text size="sm"><b>Country:</b> {countryDisplay(s.country, byCode)}</Text>
                      {geoBits && <Text size="sm" data-testid="signal-geo"><b>Location:</b> {geoBits}</Text>}
                      {s.geoScope && <Text size="sm"><b>Scope:</b> {s.geoScope}</Text>}
                      <Text size="sm"><b>Source count:</b> {s.sourceCount}</Text>
                      <Text size="sm"><b>First seen:</b> {fmtDate(s.firstSeenAt)}</Text>
                      <Text size="sm"><b>Last seen:</b> {fmtDate(s.lastSeenAt)}</Text>
                    </Stack>
                  </Card>
                  <Card withBorder radius="md">
                    <Title order={5} mb="xs">Assessment</Title>
                    <Group gap="xs" data-testid="signal-assessment">
                      {s.sentiment && <Badge variant="light" color={s.sentiment === "NEGATIVE" ? "red" : s.sentiment === "POSITIVE" ? "green" : "gray"}>Sentiment: {s.sentiment}</Badge>}
                      {s.influence && <Badge variant="light" color="grape">Influence: {s.influence}</Badge>}
                      {pct(s.relevance) && <Badge variant="light" color="blue">Relevance: {pct(s.relevance)}</Badge>}
                    </Group>
                    {!s.sentiment && !s.influence && s.relevance == null && <Text c="dimmed" size="sm">Not yet assessed.</Text>}
                  </Card>
                  {industries.length > 0 && (
                    <Card withBorder radius="md">
                      <Title order={5} mb="xs">Industries</Title>
                      <Group gap="xs" data-testid="signal-industries">
                        {industries.map((a) => (<Badge key={a.valueCode} variant="outline">{a.valueCode}</Badge>))}
                      </Group>
                    </Card>
                  )}
                  {entities.length > 0 && (
                    <Card withBorder radius="md">
                      <Title order={5} mb="xs">Entities</Title>
                      <Stack gap={4} data-testid="signal-entities">
                        {entities.map((a, i) => (
                          <Text key={i} size="sm">{a.valueText} <Text span c="dimmed" size="xs">· {a.valueCode}</Text></Text>
                        ))}
                      </Stack>
                    </Card>
                  )}
                  <Card withBorder radius="md">
                    <Title order={5} mb="xs">Tags</Title>
                    {s.tags.length === 0 ? (
                      <Text c="dimmed" size="sm">No tags.</Text>
                    ) : (
                      <Group gap="xs">
                        {s.tags.map((t) => (
                          <Badge key={t.code} variant="light">{t.code} · {Math.round(t.confidence * 100)}%</Badge>
                        ))}
                      </Group>
                    )}
                  </Card>
                </Stack>
              </Grid.Col>
            </Grid>
            );
            })()
          )
        }
      </AsyncBoundary>
    </>
  );
}
