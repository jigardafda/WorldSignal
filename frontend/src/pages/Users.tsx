import { Button, Group, Modal, PasswordInput, Paper, Select, Stack, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { api, type User } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

const ROLES = ["ADMIN", "EDITOR", "VIEWER"];
const STATUSES = ["ACTIVE", "SUSPENDED"];

export function Users() {
  const { user: me } = useAuth();
  const state = useAsync<User[]>(() => api.users(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: { email: "", name: "", password: "", role: "VIEWER" },
    validate: {
      email: (v) => (/^\S+@\S+$/.test(v) ? null : "Valid email required"),
      password: (v) => (v.length >= 8 ? null : "Min 8 characters"),
    },
  });

  async function create(v: typeof form.values) {
    setBusy(true);
    try {
      await api.createUser(v);
      notifications.show({ message: "User created", color: "green" });
      close(); form.reset(); state.reload();
    } catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
    finally { setBusy(false); }
  }

  async function update(id: string, input: Record<string, unknown>) {
    try { await api.updateUser(id, input); notifications.show({ message: "Updated", color: "green" }); state.reload(); }
    catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  return (
    <>
      <PageHeader title="Users" subtitle="Accounts, roles and access"
        actions={<Button leftSection={<IconPlus size={16} />} onClick={open}>Add user</Button>} />
      <Paper withBorder p="md" radius="md">
        <AsyncBoundary state={state} empty={(r) => r.length === 0}>
          {(rows) => (
            <DataTable rows={rows} getKey={(r) => r.id}
              columns={[
                { key: "email", header: "Email", render: (r) => r.email },
                { key: "name", header: "Name", render: (r) => r.name || "—" },
                { key: "role", header: "Role", render: (r) => (
                  <Select size="xs" w={110} data={ROLES} value={r.role} allowDeselect={false}
                    onChange={(v) => v && update(r.id, { role: v })} disabled={r.id === me?.id} />
                ) },
                { key: "status", header: "Status", render: (r) => (
                  r.id === me?.id ? <StatusBadge status={r.status} /> :
                  <Select size="xs" w={130} data={STATUSES} value={r.status} allowDeselect={false}
                    onChange={(v) => v && update(r.id, { status: v })} />
                ) },
                { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                { key: "actions", header: "", render: (r: User) => (
                  r.id === me?.id ? null :
                  <ConfirmButton label="Delete" message={`Delete ${r.email}?`} confirmLabel="Delete"
                    onConfirm={() => api.deleteUser(r.id)} onDone={state.reload} />
                ) },
              ]} />
          )}
        </AsyncBoundary>
      </Paper>
      <Modal opened={opened} onClose={close} title="Add user" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Email" required {...form.getInputProps("email")} data-testid="user-email" />
            <TextInput label="Name" {...form.getInputProps("name")} />
            <PasswordInput label="Password" required {...form.getInputProps("password")} data-testid="user-password" />
            <Select label="Role" data={ROLES} {...form.getInputProps("role")} />
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
