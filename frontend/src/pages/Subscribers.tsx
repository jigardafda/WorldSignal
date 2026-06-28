import { Button, Group, Paper, Stack, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { Modal } from "@mantine/core";
import { useState } from "react";
import { api, type Subscriber } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

export function Subscribers() {
  const { hasPerm } = useAuth();
  const canWrite = hasPerm("subscriptions:write");
  const state = useAsync<Subscriber[]>(() => api.subscribers(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const form = useForm({ initialValues: { name: "" }, validate: { name: (v) => (v.trim() ? null : "Name is required") } });

  async function create(v: { name: string }) {
    setBusy(true);
    try {
      await api.createSubscriber(v.name);
      notifications.show({ message: "Subscriber created", color: "green" });
      close(); form.reset(); state.reload();
    } catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
    finally { setBusy(false); }
  }

  return (
    <>
      <PageHeader title="Subscribers" subtitle="Tenants that own subscriptions"
        actions={canWrite && <Button leftSection={<IconPlus size={16} />} onClick={open}>Add subscriber</Button>} />
      <Paper withBorder p="md" radius="md">
        <AsyncBoundary state={state} empty={(r) => r.length === 0}>
          {(rows) => (
            <DataTable rows={rows} getKey={(r) => r.id}
              columns={[
                { key: "name", header: "Name", render: (r) => r.name },
                { key: "status", header: "Status", render: (r) => <StatusBadge status={r.status} /> },
                { key: "subscriptionCount", header: "Subscriptions", render: (r) => r.subscriptionCount },
                { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                ...(canWrite ? [{
                  key: "actions", header: "", render: (r: Subscriber) => (
                    <ConfirmButton label="Delete" message={`Delete subscriber "${r.name}" and its subscriptions?`} confirmLabel="Delete"
                      onConfirm={() => api.deleteSubscriber(r.id)} onDone={state.reload} />
                  ),
                }] : []),
              ]} />
          )}
        </AsyncBoundary>
      </Paper>
      <Modal opened={opened} onClose={close} title="Add subscriber" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" required {...form.getInputProps("name")} data-testid="subscriber-name" />
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
