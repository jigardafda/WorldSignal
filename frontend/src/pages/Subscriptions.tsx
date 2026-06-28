import { Button, Group, JsonInput, Modal, Paper, Select, Stack, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { api, type Subscription } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

function parseJSON(s: string): unknown {
  try { return s.trim() ? JSON.parse(s) : {}; } catch { return null; }
}

export function Subscriptions() {
  const { hasPerm } = useAuth();
  const canWrite = hasPerm("subscriptions:write");
  const state = useAsync<Subscription[]>(() => api.subscriptions(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: { name: "", channel: "WEBHOOK", filter: "{}", config: '{"url":""}' },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      filter: (v) => (parseJSON(v) === null ? "Invalid JSON" : null),
      config: (v) => (parseJSON(v) === null ? "Invalid JSON" : null),
    },
  });

  async function create(v: typeof form.values) {
    setBusy(true);
    try {
      await api.createSubscription({ name: v.name, channel: v.channel, filter: parseJSON(v.filter), config: parseJSON(v.config) });
      notifications.show({ message: "Subscription created", color: "green" });
      close(); form.reset(); state.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally { setBusy(false); }
  }

  async function toggle(s: Subscription) {
    try { await api.updateSubscription(s.id, { enabled: !s.enabled }); state.reload(); }
    catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  return (
    <>
      <PageHeader title="Subscriptions" subtitle="Delivery routes for matched signals"
        actions={canWrite && <Button leftSection={<IconPlus size={16} />} onClick={open}>Add subscription</Button>} />
      <Paper withBorder p="md" radius="md">
        <AsyncBoundary state={state} empty={(r) => r.length === 0}>
          {(rows) => (
            <DataTable rows={rows} getKey={(r) => r.id}
              columns={[
                { key: "name", header: "Name", render: (r) => r.name },
                { key: "channel", header: "Channel", render: (r) => r.channel },
                { key: "enabled", header: "Status", render: (r) => <StatusBadge status={r.enabled ? "ACTIVE" : "SUSPENDED"} /> },
                { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                ...(canWrite ? [{
                  key: "actions", header: "", render: (r: Subscription) => (
                    <Group gap="xs" wrap="nowrap">
                      <Button size="xs" variant="light" color={r.enabled ? "orange" : "green"} onClick={() => toggle(r)}>
                        {r.enabled ? "Disable" : "Enable"}
                      </Button>
                      <ConfirmButton label="Delete" message={`Delete subscription "${r.name}"?`} confirmLabel="Delete"
                        onConfirm={() => api.deleteSubscription(r.id)} onDone={state.reload} />
                    </Group>
                  ),
                }] : []),
              ]} />
          )}
        </AsyncBoundary>
      </Paper>

      <Modal opened={opened} onClose={close} title="Add subscription" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" required {...form.getInputProps("name")} data-testid="sub-name" />
            <Select label="Channel" data={["WEBHOOK", "POLLING", "EMAIL"]} {...form.getInputProps("channel")} />
            <JsonInput label="Filter" autosize minRows={3} formatOnBlur {...form.getInputProps("filter")} />
            <JsonInput label="Config" autosize minRows={3} formatOnBlur {...form.getInputProps("config")} />
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
