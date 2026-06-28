import { Button, Card, Code, Stack, Text } from "@mantine/core";
import { ExtLink } from "../components/ExtLink";
import { IconArrowLeft } from "@tabler/icons-react";
import { useNavigate, useParams } from "react-router-dom";
import { api, type RawItemDetail as RD } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

export function RawItemDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const state = useAsync<RD | null>(() => api.rawItem(id), [id]);
  return (
    <>
      <PageHeader title="Raw Item" actions={<Button variant="default" leftSection={<IconArrowLeft size={16} />} onClick={() => navigate("/raw-items")}>Back</Button>} />
      <AsyncBoundary state={state}>
        {(r) => (r === null ? <EmptyState message="Raw item not found." /> : (
          <Card withBorder radius="md">
            <Stack gap={6}>
              <Text fw={700} size="lg">{r.rawTitle ?? "(untitled)"}</Text>
              <Text size="sm"><b>Status:</b> <StatusBadge status={r.status} /></Text>
              <Text size="sm"><b>Source:</b> {r.sourceName}</Text>
              <Text size="sm"><b>GUID:</b> {r.sourceGuid ?? "—"}</Text>
              {r.rawUrl && <Text size="sm"><b>URL:</b> <ExtLink url={r.rawUrl} /></Text>}
              <Text size="sm"><b>Published:</b> {fmtDate(r.publishedAt)} · <b>Fetched:</b> {fmtDate(r.fetchedAt)}</Text>
              <Text size="sm" fw={700} mt="sm">Content</Text>
              <Text size="sm" c="dimmed">{r.rawContent || "—"}</Text>
              <Text size="sm" fw={700} mt="sm">Raw payload</Text>
              <Code block>{JSON.stringify(r.rawPayload, null, 2)}</Code>
            </Stack>
          </Card>
        ))}
      </AsyncBoundary>
    </>
  );
}
