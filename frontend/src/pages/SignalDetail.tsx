import { Badge, Button, Card, Grid, Group, List, Paper, Stack, Text, Title } from "@mantine/core";
import { ExtLink } from "../components/ExtLink";
import { IconArrowLeft } from "@tabler/icons-react";
import { useNavigate, useParams } from "react-router-dom";
import { api, type Signal } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { ConfidenceBar, SeverityBadge, StatusBadge } from "../components/badges";
import { useCountries, countryDisplay } from "../lib/countries";
import { fmtDate } from "../lib/format";

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
            <Grid>
              <Grid.Col span={{ base: 12, md: 8 }}>
                <Paper withBorder p="lg" radius="md">
                  <Group mb="sm">
                    <SeverityBadge severity={s.severity} />
                    <StatusBadge status={s.status} />
                    {s.eventType && <Badge variant="outline">{s.eventType}</Badge>}
                  </Group>
                  <Title order={3}>{s.title}</Title>
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
                      <Text size="sm"><b>Source count:</b> {s.sourceCount}</Text>
                      <Text size="sm"><b>First seen:</b> {fmtDate(s.firstSeenAt)}</Text>
                      <Text size="sm"><b>Last seen:</b> {fmtDate(s.lastSeenAt)}</Text>
                    </Stack>
                  </Card>
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
          )
        }
      </AsyncBoundary>
    </>
  );
}
