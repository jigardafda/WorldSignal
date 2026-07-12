import { Badge, Button, Group, Modal, Paper, Select, Stack, Text, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { api, type Subscription } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

const CHANNELS = ["WEBHOOK", "EMAIL", "POLLING", "SSE", "WEBSOCKET"];

function notifyError(e: unknown) {
  notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
}

// MySubscriptions is the customer-console delivery manager: a tenant creates and
// manages its own subscriptions (what signals it receives and where), scoped to
// its account.
export function MySubscriptions() {
  const state = useAsync<Subscription[]>(() => api.mySubscriptions(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: { name: "", channel: "WEBHOOK", url: "" },
    validate: { name: (v) => (v.trim() ? null : "Name is required") },
  });

  async function create(v: { name: string; channel: string; url: string }) {
    setBusy(true);
    try {
      const config = v.url.trim() ? { url: v.url.trim() } : {};
      await api.createMySubscription({ name: v.name, channel: v.channel, filter: {}, config });
      notifications.show({ message: "Subscription created", color: "green" });
      close();
      form.reset();
      state.reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setBusy(false);
    }
  }

  async function toggle(sub: Subscription) {
    try {
      await api.updateMySubscription(sub.id, { enabled: !sub.enabled });
      state.reload();
    } catch (e) {
      notifyError(e);
    }
  }

  return (
    <>
      <PageHeader
        title="Subscriptions"
        subtitle="Choose what signals you receive and where they're delivered"
        actions={<Button leftSection={<IconPlus size={16} />} onClick={open} data-testid="add-subscription">New subscription</Button>}
      />
      <Paper withBorder p="md" radius="lg">
        <AsyncBoundary state={state} empty={(r) => r.length === 0}>
          {(rows) => (
            <DataTable
              rows={rows}
              getKey={(r) => r.id}
              columns={[
                { key: "name", header: "Name", render: (r) => r.name },
                { key: "channel", header: "Channel", render: (r) => <Badge variant="light">{r.channel}</Badge> },
                { key: "enabled", header: "Status", render: (r) => <StatusBadge status={r.enabled ? "ACTIVE" : "SUSPENDED"} /> },
                { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                {
                  key: "actions", header: "", render: (r: Subscription) => (
                    <Group gap="xs" wrap="nowrap">
                      <Button size="xs" variant="light" color={r.enabled ? "orange" : "green"} onClick={() => toggle(r)} data-testid={`toggle-${r.id}`}>
                        {r.enabled ? "Pause" : "Resume"}
                      </Button>
                      <ConfirmButton label="Delete" message={`Delete subscription "${r.name}"?`} confirmLabel="Delete"
                        onConfirm={() => api.deleteMySubscription(r.id)} onDone={state.reload} />
                    </Group>
                  ),
                },
              ]}
            />
          )}
        </AsyncBoundary>
      </Paper>
      <Modal opened={opened} onClose={close} title="New subscription" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" required {...form.getInputProps("name")} data-testid="subscription-name" />
            <Select label="Delivery channel" data={CHANNELS} allowDeselect={false} {...form.getInputProps("channel")} data-testid="subscription-channel" />
            <TextInput label="Destination URL" description="For WEBHOOK delivery (optional)" placeholder="https://example.com/hook" {...form.getInputProps("url")} data-testid="subscription-url" />
            <Text size="xs" c="dimmed">Signals matching your filters are delivered to this subscription. Refine filters from the signals view.</Text>
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
