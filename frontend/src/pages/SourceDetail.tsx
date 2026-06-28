import { Badge, Button, Card, Grid, Group, NumberInput, Stack, Switch, Text, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { IconArrowLeft, IconRefresh } from "@tabler/icons-react";
import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api, type Source } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { HealthBadge, StatusBadge, ValidationBadge } from "../components/badges";
import { ExtLink } from "../components/ExtLink";
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
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      crawlFrequency: (v) => (v >= 30 ? null : "Must be at least 30 seconds"),
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

  async function revalidate() {
    setBusy(true);
    try {
      const r = await api.revalidateSource(source.id);
      notifications.show({ message: r.validationStatus === "VALID" ? "Revalidated — VALID" : `Revalidated — ${r.validationStatus}`, color: r.validationStatus === "VALID" ? "green" : "red" });
      reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally {
      setBusy(false);
    }
  }

  const logs = source.validationLogs ?? [];

  return (
    <Stack>
    <Grid>
      <Grid.Col span={{ base: 12, md: 6 }}>
        <Card withBorder radius="md">
          <Group justify="space-between" mb="xs">
            <Text fw={700}>{source.name}</Text>
            <Group gap="xs">
              <HealthBadge score={source.healthScore} />
              <ValidationBadge status={source.validationStatus} />
              <StatusBadge status={source.enabled ? "ACTIVE" : "SUSPENDED"} />
            </Group>
          </Group>
          <Stack gap={4}>
            <Text size="sm"><b>Feed:</b> <ExtLink url={source.url} size="sm">{source.url}</ExtLink></Text>
            {source.websiteUrl && <Text size="sm"><b>Website:</b> <ExtLink url={source.websiteUrl} size="sm">{source.websiteUrl}</ExtLink></Text>}
            <Text size="sm"><b>Publisher:</b> {source.publisher ?? "—"} · <b>Org:</b> {source.orgType ?? "—"}{source.officialFeed ? " · official" : ""}</Text>
            <Text size="sm"><b>Type:</b> {source.sourceType ?? source.type} · <b>Parser:</b> {source.parserType}</Text>
            <Text size="sm"><b>Scope:</b> {source.geographicScope ?? "—"} · <b>Country:</b> {source.country ?? "—"} · <b>Region:</b> {source.region ?? "—"}</Text>
            <Text size="sm"><b>Languages:</b> {(source.languages ?? []).join(", ") || (source.language ?? "—")}</Text>
            <Text size="sm"><b>Category:</b> {source.category ?? "—"} · <b>Industry:</b> {source.industry ?? "—"}{source.subcategory ? ` · ${source.subcategory}` : ""}</Text>
            <Text size="sm"><b>Priority:</b> P{source.priority} · <b>Credibility:</b> {pct(source.credibility)} · <b>Crawl:</b> {source.crawlFrequency}s</Text>
            {source.tags && source.tags.length > 0 && (
              <Group gap={4} mt={4}>{source.tags.map((t) => <Badge key={t} size="sm" variant="outline" color="gray">{t}</Badge>)}</Group>
            )}
          </Stack>
        </Card>
        <Card withBorder radius="md" mt="md">
          <Text fw={700} mb="xs">Validation & health</Text>
          <Stack gap={4}>
            <Text size="sm"><b>Health score:</b> {source.healthScore ?? "—"}/100 · <b>Avg response:</b> {source.avgResponseMs != null ? `${source.avgResponseMs} ms` : "—"}</Text>
            <Text size="sm"><b>Last validated:</b> {fmtDate(source.lastValidatedAt)}</Text>
            <Text size="sm"><b>Last success:</b> {fmtDate(source.lastSuccessAt)} · <b>Last failure:</b> {fmtDate(source.lastFailureAt)}</Text>
            <Text size="sm"><b>Failures:</b> {source.failureCount}</Text>
            {source.lastValidationError && <Text size="sm" c="red"><b>Last error:</b> {source.lastValidationError}</Text>}
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
                    <Button variant="light" color="grape" leftSection={<IconRefresh size={16} />} loading={busy} onClick={revalidate} data-testid="revalidate">Revalidate</Button>
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

    <Card withBorder radius="md">
      <Group justify="space-between" mb="xs">
        <Text fw={700}>Validation history</Text>
        {!canWrite && <Button variant="light" color="grape" size="xs" leftSection={<IconRefresh size={14} />} loading={busy} onClick={revalidate} data-testid="revalidate">Revalidate</Button>}
      </Group>
      {logs.length === 0 ? (
        <Text c="dimmed" size="sm">No validation runs recorded yet.</Text>
      ) : (
        <DataTable
          rows={logs}
          getKey={(l) => l.id}
          columns={[
            { key: "checkedAt", header: "Checked", render: (l) => fmtDate(l.checkedAt) },
            { key: "ok", header: "Result", render: (l) => <ValidationBadge status={l.ok ? "VALID" : "INVALID"} /> },
            { key: "httpStatus", header: "HTTP", render: (l) => l.httpStatus ?? "—" },
            { key: "responseMs", header: "Latency", render: (l) => (l.responseMs != null ? `${l.responseMs} ms` : "—") },
            { key: "itemCount", header: "Items", render: (l) => l.itemCount ?? "—" },
            { key: "newestItemAt", header: "Newest item", render: (l) => fmtDate(l.newestItemAt) },
            { key: "error", header: "Error", render: (l) => l.error ?? "—" },
          ]}
        />
      )}
    </Card>
    </Stack>
  );
}
