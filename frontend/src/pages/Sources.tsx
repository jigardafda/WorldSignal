import { Button, Group, Modal, NumberInput, Paper, Select, Stack, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Source } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { StatusBadge } from "../components/badges";
import { fmtDate, pct } from "../lib/format";

export function Sources() {
  const navigate = useNavigate();
  const { hasPerm } = useAuth();
  const canWrite = hasPerm("sources:write");
  const state = useAsync<Source[]>(() => api.sources(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);

  const form = useForm({
    initialValues: { name: "", url: "", country: "", type: "RSS", priority: 2, crawlFrequency: 900, credibility: 0.5 },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      url: (v) => (/^https?:\/\//.test(v) ? null : "Enter a valid http(s) URL"),
    },
  });

  async function create(values: typeof form.values) {
    setBusy(true);
    try {
      await api.createSource({
        name: values.name, url: values.url, type: values.type,
        country: values.country || null, priority: values.priority,
        crawlFrequency: values.crawlFrequency, credibility: values.credibility,
      });
      notifications.show({ message: "Source created", color: "green" });
      close();
      form.reset();
      state.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally {
      setBusy(false);
    }
  }

  async function act(fn: () => Promise<unknown>, msg: string) {
    try {
      await fn();
      notifications.show({ message: msg, color: "green" });
      state.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    }
  }

  return (
    <>
      <PageHeader
        title="Sources"
        subtitle="RSS/Atom feeds the pipeline ingests"
        actions={canWrite && <Button leftSection={<IconPlus size={16} />} onClick={open}>Add source</Button>}
      />
      <Paper withBorder p="md" radius="md">
        <AsyncBoundary state={state} empty={(rows) => rows.length === 0}>
          {(rows) => (
            <DataTable
              rows={rows}
              getKey={(r) => r.id}
              onRowClick={(r) => navigate(`/sources/${r.id}`)}
              columns={[
                { key: "name", header: "Name", render: (r) => r.name },
                { key: "country", header: "Country", render: (r) => r.country ?? "—" },
                { key: "priority", header: "Priority", render: (r) => `P${r.priority}` },
                { key: "credibility", header: "Credibility", render: (r) => pct(r.credibility) },
                { key: "enabled", header: "Status", render: (r) => <StatusBadge status={r.enabled ? "ACTIVE" : "SUSPENDED"} /> },
                { key: "lastSuccessAt", header: "Last success", render: (r) => fmtDate(r.lastSuccessAt) },
                { key: "failureCount", header: "Fails", render: (r) => r.failureCount },
                ...(canWrite ? [{
                  key: "actions", header: "", render: (r: Source) => (
                    <Group gap="xs" onClick={(e) => e.stopPropagation()} wrap="nowrap">
                      <Button size="xs" variant="light" onClick={() => act(() => api.triggerFetch(r.id), `Queued ${r.name}`)}>Fetch</Button>
                      <Button size="xs" variant="light" color={r.enabled ? "orange" : "green"}
                        onClick={() => act(() => api.setSourceEnabled(r.id, !r.enabled), "Updated")}>
                        {r.enabled ? "Disable" : "Enable"}
                      </Button>
                      <ConfirmButton label="Delete" message={`Delete source "${r.name}"? This removes its raw items and articles.`}
                        confirmLabel="Delete" onConfirm={() => api.deleteSource(r.id)} onDone={state.reload} />
                    </Group>
                  ),
                }] : []),
              ]}
            />
          )}
        </AsyncBoundary>
      </Paper>

      <Modal opened={opened} onClose={close} title="Add source" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" required {...form.getInputProps("name")} data-testid="src-name" />
            <TextInput label="RSS/Atom URL" required {...form.getInputProps("url")} data-testid="src-url" />
            <TextInput label="Country" placeholder="US" {...form.getInputProps("country")} />
            <Select label="Type" data={["RSS", "ATOM"]} {...form.getInputProps("type")} />
            <NumberInput label="Priority (0=highest)" min={0} max={5} {...form.getInputProps("priority")} />
            <NumberInput label="Crawl frequency (s)" min={30} {...form.getInputProps("crawlFrequency")} />
            <NumberInput label="Credibility" min={0} max={1} step={0.05} decimalScale={2} {...form.getInputProps("credibility")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>Cancel</Button>
              <Button type="submit" loading={busy}>Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </>
  );
}
