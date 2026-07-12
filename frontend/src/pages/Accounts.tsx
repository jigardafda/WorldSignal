import { Badge, Button, Group, Modal, Paper, Select, Stack, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { api, type Account } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

const PLANS = ["FREE", "PRO", "ENTERPRISE"];

function notifyError(e: unknown) {
  notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
}

export function Accounts() {
  const { hasPerm } = useAuth();
  const canManage = hasPerm("accounts:manage");
  const state = useAsync<Account[]>(() => api.accounts(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: { name: "", slug: "", plan: "FREE" },
    validate: { name: (v) => (v.trim() ? null : "Name is required") },
  });

  async function create(v: { name: string; slug: string; plan: string }) {
    setBusy(true);
    try {
      await api.createAccount({ name: v.name, slug: v.slug || undefined, plan: v.plan });
      notifications.show({ message: "Account created", color: "green" });
      close();
      form.reset();
      state.reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setBusy(false);
    }
  }

  async function toggleStatus(a: Account) {
    const status = a.status === "SUSPENDED" ? "ACTIVE" : "SUSPENDED";
    try {
      await api.updateAccount(a.id, { status });
      notifications.show({ message: `Account ${status === "ACTIVE" ? "activated" : "suspended"}`, color: "green" });
      state.reload();
    } catch (e) {
      notifyError(e);
    }
  }

  return (
    <>
      <PageHeader
        title="Accounts"
        subtitle="SaaS tenants — each owns its API keys and subscriptions over the shared signal pool"
        actions={canManage && <Button leftSection={<IconPlus size={16} />} onClick={open}>Add account</Button>}
      />
      <Paper withBorder p="md" radius="md">
        <AsyncBoundary state={state} empty={(r) => r.length === 0}>
          {(rows) => (
            <DataTable
              rows={rows}
              getKey={(r) => r.id}
              columns={[
                { key: "name", header: "Name", render: (r) => r.name },
                { key: "slug", header: "Slug", render: (r) => <code>{r.slug}</code> },
                { key: "plan", header: "Plan", render: (r) => <Badge variant="light">{r.plan}</Badge> },
                { key: "status", header: "Status", render: (r) => <StatusBadge status={r.status} /> },
                { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                ...(canManage
                  ? [{
                      key: "actions",
                      header: "",
                      render: (r: Account) => (
                        <Button
                          size="xs"
                          variant="light"
                          color={r.status === "SUSPENDED" ? "green" : "orange"}
                          onClick={() => toggleStatus(r)}
                          data-testid={`toggle-${r.id}`}
                        >
                          {r.status === "SUSPENDED" ? "Activate" : "Suspend"}
                        </Button>
                      ),
                    }]
                  : []),
              ]}
            />
          )}
        </AsyncBoundary>
      </Paper>
      <Modal opened={opened} onClose={close} title="Add account" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" required {...form.getInputProps("name")} data-testid="account-name" />
            <TextInput label="Slug" description="Optional — derived from the name if left blank" {...form.getInputProps("slug")} data-testid="account-slug" />
            <Select label="Plan" data={PLANS} allowDeselect={false} {...form.getInputProps("plan")} data-testid="account-plan" />
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
