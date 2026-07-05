import { Alert, Anchor, Button, Divider, Group, Modal, Paper, ScrollArea, Select, Stack, Text, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { api, type EmailConnector, type Subscriber, type Subscription } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";
import { FilterBuilder } from "../components/FilterBuilder";
import { CodeExamples } from "../components/CodeExamples";
import { cleanFilter, filterSummary, type SubFilter } from "../lib/subFilter";
import type { Channel } from "../lib/codegen";

const CHANNELS = [
  { value: "WEBHOOK", label: "Webhook — signed POST to your URL" },
  { value: "SSE", label: "SSE — live event stream" },
  { value: "WEBSOCKET", label: "WebSocket — live event stream" },
  { value: "POLLING", label: "Polling — pull on your own schedule" },
  { value: "EMAIL", label: "Email — delivered to recipients" },
];
const PULL = new Set(["SSE", "WEBSOCKET", "POLLING"]);

const DELIVERY_MODES = [
  { value: "instant", label: "Instant — one email per matched signal" },
  { value: "hourly", label: "Hourly digest — one rollup email per hour" },
  { value: "daily", label: "Daily digest — one rollup email per day" },
];

function safeFilter(raw: unknown): SubFilter {
  if (!raw) return {};
  try {
    return typeof raw === "string" ? (JSON.parse(raw) as SubFilter) : (raw as SubFilter);
  } catch {
    return {};
  }
}

export function Subscriptions() {
  const { hasPerm } = useAuth();
  const canWrite = hasPerm("subscriptions:write");
  const state = useAsync<Subscription[]>(() => api.subscriptions(), []);
  const subscribers = useAsync<Subscriber[]>(() => api.subscribers(), []);
  const connectors = useAsync<EmailConnector[]>(
    () => (typeof api.emailConnectors === "function" ? api.emailConnectors() : Promise.resolve([])),
    [],
  );

  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState<SubFilter>({});
  const [baseUrl, setBaseUrl] = useState("");
  const [subscriberId, setSubscriberId] = useState<string>("");

  // Default the base URL (shown in the code examples) and the subscriber picker.
  useEffect(() => {
    if (!baseUrl && typeof window !== "undefined") setBaseUrl(window.location.origin);
  }, [baseUrl]);
  useEffect(() => {
    if (!subscriberId && subscribers.data?.length) setSubscriberId(subscribers.data[0].id);
  }, [subscribers.data, subscriberId]);

  const form = useForm({
    initialValues: { name: "", channel: "WEBHOOK", url: "", connectorId: "", recipients: "", mode: "instant" },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
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

  function openNew() {
    form.reset();
    setFilter({});
    open();
  }

  async function create(v: typeof form.values) {
    setBusy(true);
    try {
      await api.createSubscription({
        name: v.name,
        channel: v.channel,
        filter: cleanFilter(filter),
        config: buildConfig(v),
        ...(subscriberId ? { subscriberId } : {}),
      });
      notifications.show({ message: "Subscription created", color: "green" });
      close();
      state.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally {
      setBusy(false);
    }
  }

  async function toggle(s: Subscription) {
    try { await api.updateSubscription(s.id, { enabled: !s.enabled }); state.reload(); }
    catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  const channel = form.values.channel as Channel;
  const connectorOptions = [
    { value: "", label: "Active connector (default)" },
    ...(connectors.data ?? []).map((c) => ({ value: c.id, label: `${c.name} (${c.fromEmail})` })),
  ];
  const noConnectors = !!connectors.data && connectors.data.length === 0;
  const subscriberOptions = (subscribers.data ?? []).map((s) => ({ value: s.id, label: s.name }));

  return (
    <>
      <PageHeader title="Subscriptions" subtitle="Delivery routes for matched signals"
        actions={canWrite && <Button leftSection={<IconPlus size={16} />} onClick={openNew}>Add subscription</Button>} />
      <Paper withBorder p="md" radius="md">
        <AsyncBoundary state={state} empty={(r) => r.length === 0}>
          {(rows) => (
            <DataTable rows={rows} getKey={(r) => r.id}
              columns={[
                { key: "name", header: "Name", render: (r) => r.name },
                { key: "channel", header: "Channel", render: (r) => r.channel },
                { key: "filter", header: "Filter", render: (r) => <Text size="sm" c="dimmed">{filterSummary(safeFilter(r.filter))}</Text> },
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

      <Modal opened={opened} onClose={close} title="New subscription" size="90%" data-testid="sub-modal"
        closeOnClickOutside={false} /* dropdowns portal outside the modal; don't treat those clicks as "close" */>
        <form onSubmit={form.onSubmit(create)}>
          <div style={{ display: "flex", gap: 24, minHeight: "72vh" }}>
            {/* Left: configuration */}
            <ScrollArea.Autosize style={{ flex: "0 0 420px" }} mah="74vh" pr="sm" data-testid="sub-config">
              <Stack gap="sm">
                {subscriberOptions.length > 0 && (
                  <Select label="Subscriber" data={subscriberOptions} value={subscriberId} onChange={(v) => setSubscriberId(v ?? "")} data-testid="sub-subscriber" />
                )}
                <TextInput label="Name" required {...form.getInputProps("name")} data-testid="sub-name" />
                <Select label="Delivery channel" data={CHANNELS} allowDeselect={false} {...form.getInputProps("channel")} data-testid="sub-channel" />

                {channel === "WEBHOOK" && (
                  <TextInput label="Webhook URL" placeholder="https://example.com/hooks/worldsignal" {...form.getInputProps("url")} data-testid="sub-url" />
                )}
                {PULL.has(channel) && (
                  <Alert color="blue" py="xs" variant="light">
                    Your client connects to this subscription with an API key (<Anchor href="/api-keys">API Keys</Anchor>, scope <code>signals:read</code>). Copy the code on the right.
                  </Alert>
                )}
                {channel === "EMAIL" && (
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
                    <Select label="Delivery" data={DELIVERY_MODES} allowDeselect={false} {...form.getInputProps("mode")} data-testid="sub-mode" />
                  </>
                )}

                <Divider label="Filter" labelPosition="left" mt="xs" />
                <FilterBuilder value={filter} onChange={setFilter} />
              </Stack>
            </ScrollArea.Autosize>

            <Divider orientation="vertical" />

            {/* Right: live code examples */}
            <div style={{ flex: 1, minWidth: 0 }}>
              <Group justify="space-between" mb="xs" wrap="nowrap">
                <Text fw={600} size="sm">Consume it — {filterSummary(cleanFilter(filter))}</Text>
                <TextInput size="xs" label={undefined} placeholder="API base URL" style={{ width: 260 }}
                  value={baseUrl} onChange={(e) => setBaseUrl(e.currentTarget.value)} data-testid="sub-baseurl" />
              </Group>
              <CodeExamples channel={channel} opts={{ baseUrl, subscriptionId: "<your-subscription-id>" }} />
            </div>
          </div>

          <Group justify="flex-end" mt="md">
            <Button variant="default" onClick={close}>Cancel</Button>
            <Button type="submit" loading={busy} data-testid="sub-create">Create subscription</Button>
          </Group>
        </form>
      </Modal>
    </>
  );
}
