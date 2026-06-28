import { Button, Card, Grid, Group, NumberInput, Stack, Switch, Text, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { IconArrowLeft } from "@tabler/icons-react";
import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api, type Source } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { ConfirmButton } from "../components/ConfirmButton";
import { StatusBadge } from "../components/badges";
import { fmtDate, pct } from "../lib/format";

export function SourceDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { hasPerm } = useAuth();
  const canWrite = hasPerm("sources:write");
  const state = useAsync<Source | null>(() => api.source(id), [id]);

  return (
    <>
      <PageHeader title="Source" actions={<Button variant="default" leftSection={<IconArrowLeft size={16} />} onClick={() => navigate("/sources")}>Back</Button>} />
      <AsyncBoundary state={state}>
        {(s) => (s === null ? <EmptyState message="Source not found." /> : <SourceBody source={s} canWrite={canWrite} reload={state.reload} navigate={navigate} />)}
      </AsyncBoundary>
    </>
  );
}

function SourceBody({ source, canWrite, reload, navigate }: { source: Source; canWrite: boolean; reload: () => void; navigate: (to: string) => void }) {
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: {
      name: source.name, country: source.country ?? "", priority: source.priority,
      credibility: source.credibility, crawlFrequency: source.crawlFrequency, enabled: source.enabled,
    },
  });

  async function save(v: typeof form.values) {
    setBusy(true);
    try {
      await api.updateSource(source.id, { name: v.name, country: v.country || null, priority: v.priority, credibility: v.credibility, crawlFrequency: v.crawlFrequency, enabled: v.enabled });
      notifications.show({ message: "Saved", color: "green" });
      reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Grid>
      <Grid.Col span={{ base: 12, md: 6 }}>
        <Card withBorder radius="md">
          <Group justify="space-between" mb="xs">
            <Text fw={700}>{source.name}</Text>
            <StatusBadge status={source.enabled ? "ACTIVE" : "SUSPENDED"} />
          </Group>
          <Stack gap={4}>
            <Text size="sm"><b>URL:</b> {source.url}</Text>
            <Text size="sm"><b>Type:</b> {source.type} · <b>Parser:</b> {source.parserType}</Text>
            <Text size="sm"><b>Country:</b> {source.country ?? "—"} · <b>Region:</b> {source.region ?? "—"}</Text>
            <Text size="sm"><b>Priority:</b> P{source.priority} · <b>Credibility:</b> {pct(source.credibility)}</Text>
            <Text size="sm"><b>Crawl frequency:</b> {source.crawlFrequency}s</Text>
            <Text size="sm"><b>Failures:</b> {source.failureCount}</Text>
            <Text size="sm"><b>Last success:</b> {fmtDate(source.lastSuccessAt)}</Text>
            <Text size="sm"><b>Last failure:</b> {fmtDate(source.lastFailureAt)}</Text>
            <Text size="sm"><b>Last fetched:</b> {fmtDate(source.lastFetchedAt)}</Text>
            <Text size="sm"><b>Created:</b> {fmtDate(source.createdAt)}</Text>
          </Stack>
        </Card>
      </Grid.Col>
      {canWrite && (
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder radius="md">
            <Text fw={700} mb="sm">Edit</Text>
            <form onSubmit={form.onSubmit(save)}>
              <Stack>
                <TextInput label="Name" {...form.getInputProps("name")} />
                <TextInput label="Country" {...form.getInputProps("country")} />
                <NumberInput label="Priority" min={0} max={5} {...form.getInputProps("priority")} />
                <NumberInput label="Credibility" min={0} max={1} step={0.05} decimalScale={2} {...form.getInputProps("credibility")} />
                <NumberInput label="Crawl frequency (s)" min={30} {...form.getInputProps("crawlFrequency")} />
                <Switch label="Enabled" checked={form.values.enabled} {...form.getInputProps("enabled", { type: "checkbox" })} />
                <Group justify="space-between">
                  <ConfirmButton label="Delete source" message={`Delete "${source.name}"?`} confirmLabel="Delete"
                    onConfirm={() => api.deleteSource(source.id)} onDone={() => navigate("/sources")} />
                  <Group>
                    <Button variant="light" onClick={() => api.triggerFetch(source.id).then(() => notifications.show({ message: "Fetch queued", color: "green" }))}>Fetch now</Button>
                    <Button type="submit" loading={busy}>Save</Button>
                  </Group>
                </Group>
              </Stack>
            </form>
          </Card>
        </Grid.Col>
      )}
    </Grid>
  );
}
