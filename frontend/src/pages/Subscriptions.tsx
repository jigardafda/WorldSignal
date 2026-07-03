import { Alert, Anchor, Button, Group, JsonInput, Modal, Paper, Select, Stack, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { api, type EmailConnector, type Subscription } from "../lib/api";
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

const DELIVERY_MODES = [
  { value: "instant", label: "Instant — one email per matched signal" },
  { value: "hourly", label: "Hourly digest — one rollup email per hour" },
  { value: "daily", label: "Daily digest — one rollup email per day" },
];

export function Subscriptions() {
  const { hasPerm } = useAuth();
  const canWrite = hasPerm("subscriptions:write");
  const state = useAsync<Subscription[]>(() => api.subscriptions(), []);
  // Connectors power the EMAIL channel picker. The query needs settings:manage,
  // so non-admins simply fall back to the active connector (empty list is fine).
  const connectors = useAsync<EmailConnector[]>(
    () => (typeof api.emailConnectors === "function" ? api.emailConnectors() : Promise.resolve([])),
    [],
  );
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: {
      name: "", channel: "WEBHOOK", filter: "{}",
      url: "", connectorId: "", recipients: "", mode: "instant",
    },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      filter: (v) => (parseJSON(v) === null ? "Invalid JSON" : null),
      recipients: (v, values) => (values.channel === "EMAIL" && !v.trim() ? "At least one recipient is required" : null),
    },
  });

  function buildConfig(v: typeof form.values): unknown {
    if (v.channel === "WEBHOOK") return { url: v.url };
    if (v.channel === "EMAIL") {
      const cfg: Record<string, unknown> = { to: v.recipients };
      if (v.connectorId) cfg.connectorId = v.connectorId;
      if (v.mode === "hourly" || v.mode === "daily") { cfg.mode = "digest"; cfg.interval = v.mode; }
      else cfg.mode = "instant";
      return cfg;
    }
    return {};
  }

  async function create(v: typeof form.values) {
    setBusy(true);
    try {
      await api.createSubscription({ name: v.name, channel: v.channel, filter: parseJSON(v.filter), config: buildConfig(v) });
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

  const connectorOptions = [
    { value: "", label: "Active connector (default)" },
    ...(connectors.data ?? []).map((c) => ({ value: c.id, label: `${c.name} (${c.fromEmail})` })),
  ];
  const noConnectors = !!connectors.data && connectors.data.length === 0;

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
            <Select label="Channel" data={["WEBHOOK", "POLLING", "EMAIL"]} {...form.getInputProps("channel")} allowDeselect={false} data-testid="sub-channel" />

            {form.values.channel === "WEBHOOK" && (
              <TextInput label="Webhook URL" placeholder="https://example.com/hooks/worldsignal" {...form.getInputProps("url")} data-testid="sub-url" />
            )}

            {form.values.channel === "EMAIL" && (
              <>
                {noConnectors && (
                  <Alert color="orange" py="xs">
                    No email connector is configured. Add one under <Anchor href="/connectors">Connectors</Anchor> first, or an admin must set one active.
                  </Alert>
                )}
                <TextInput label="Recipients" placeholder="alerts@team.com, ops@team.com"
                  description="Comma-separated email addresses" {...form.getInputProps("recipients")} data-testid="sub-recipients" />
                {(connectors.data?.length ?? 0) > 0 && (
                  <Select label="Connector" data={connectorOptions} {...form.getInputProps("connectorId")} data-testid="sub-connector" />
                )}
                <Select label="Delivery" data={DELIVERY_MODES} {...form.getInputProps("mode")} allowDeselect={false} data-testid="sub-mode" />
              </>
            )}

            <JsonInput label="Filter (JSON)" description='e.g. {"tags":["DISASTER"],"minSeverity":"HIGH","countries":["US"]}'
              autosize minRows={3} formatOnBlur {...form.getInputProps("filter")} />
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
