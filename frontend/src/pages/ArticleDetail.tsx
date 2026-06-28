import { Button, Card, Code, Grid, Stack, Text } from "@mantine/core";
import { ExtLink } from "../components/ExtLink";
import { IconArrowLeft } from "@tabler/icons-react";
import { useNavigate, useParams } from "react-router-dom";
import { api, type ArticleDetail as AD } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { fmtDate } from "../lib/format";

export function ArticleDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const state = useAsync<AD | null>(() => api.article(id), [id]);
  return (
    <>
      <PageHeader title="Article" actions={<Button variant="default" leftSection={<IconArrowLeft size={16} />} onClick={() => navigate("/articles")}>Back</Button>} />
      <AsyncBoundary state={state}>
        {(a) => (a === null ? <EmptyState message="Article not found." /> : (
          <Grid>
            <Grid.Col span={{ base: 12, md: 8 }}>
              <Card withBorder radius="md">
                <Text fw={700} size="lg">{a.title}</Text>
                {a.canonicalUrl && <ExtLink url={a.canonicalUrl} size="sm" />}
                <Text mt="md">{a.body || a.summary || "No body."}</Text>
                <Text fw={700} mt="lg" mb="xs">Linked signals</Text>
                <DataTable rows={a.signals} getKey={(s) => s.id} emptyMessage="Not linked to any signal."
                  onRowClick={(s) => navigate(`/signals/${s.id}`)}
                  columns={[
                    { key: "title", header: "Signal", render: (s) => s.title },
                    { key: "relationType", header: "Relation", render: (s) => s.relationType },
                    { key: "similarityScore", header: "Similarity", render: (s) => s.similarityScore == null ? "—" : s.similarityScore.toFixed(2) },
                  ]} />
              </Card>
            </Grid.Col>
            <Grid.Col span={{ base: 12, md: 4 }}>
              <Card withBorder radius="md">
                <Text fw={700} mb="xs">Metadata</Text>
                <Stack gap={4}>
                  <Text size="sm"><b>Source:</b> {a.sourceName}</Text>
                  <Text size="sm"><b>Author:</b> {a.author ?? "—"}</Text>
                  <Text size="sm"><b>Language:</b> {a.language ?? "—"} · <b>Country:</b> {a.country ?? "—"}</Text>
                  <Text size="sm"><b>Published:</b> {fmtDate(a.publishedAt)}</Text>
                  <Text size="sm"><b>Fetched:</b> {fmtDate(a.fetchedAt)}</Text>
                  <Text size="sm"><b>Content hash:</b></Text>
                  <Code block>{a.contentHash ?? "—"}</Code>
                </Stack>
              </Card>
            </Grid.Col>
          </Grid>
        ))}
      </AsyncBoundary>
    </>
  );
}
