import {
  Alert, Anchor, Badge, Button, Group, Modal, NumberInput, Paper, PasswordInput,
  Select, Stack, Text, TextInput,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconMail, IconPlus, IconSend } from "@tabler/icons-react";
import { useState } from "react";
import { api, type EmailConnector, type EmailProvider } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { ValidationBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

const SECURITY = [
  { value: "STARTTLS", label: "STARTTLS (port 587)" },
  { value: "TLS", label: "SSL/TLS (port 465)" },
  { value: "NONE", label: "None (unencrypted — testing only)" },
];

interface FormValues {
  name: string; provider: string; host: string; port: number; security: string;
  username: string; secret: string; fromEmail: string; fromName: string;
}

export function Connectors() {
  const connectors = useAsync<EmailConnector[]>(() => api.emailConnectors(), []);
  const providers = useAsync<EmailProvider[]>(() => api.emailProviders(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [editing, setEditing] = useState<EmailConnector | null>(null);
  const [busy, setBusy] = useState(false);

  const form = useForm<FormValues>({
    initialValues: { name: "", provider: "GMAIL", host: "smtp.gmail.com", port: 587, security: "STARTTLS", username: "", secret: "", fromEmail: "", fromName: "WorldSignal" },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      host: (v) => (v.trim() ? null : "Host is required"),
      port: (v) => (v > 0 && v <= 65535 ? null : "Invalid port"),
      fromEmail: (v) => (/.+@.+\..+/.test(v) ? null : "A valid from address is required"),
      secret: (v) => (editing || v.trim() ? null : "A password / API key is required"),
    },
  });

  const preset = providers.data?.find((p) => p.code === form.values.provider);

  function pickProvider(code: string) {
    const p = providers.data?.find((x) => x.code === code);
    form.setFieldValue("provider", code);
    if (p && !editing) {
      // Pre-fill transport defaults from the preset (custom leaves them editable).
      if (p.host) form.setFieldValue("host", p.host);
      form.setFieldValue("port", p.port);
      form.setFieldValue("security", p.security);
    }
  }

  function openCreate() {
    setEditing(null);
    form.setValues({ name: "", provider: "GMAIL", host: "smtp.gmail.com", port: 587, security: "STARTTLS", username: "", secret: "", fromEmail: "", fromName: "WorldSignal" });
    const gmail = providers.data?.find((p) => p.code === "GMAIL");
    if (gmail) form.setValues({ name: "", provider: "GMAIL", host: gmail.host, port: gmail.port, security: gmail.security, username: "", secret: "", fromEmail: "", fromName: "WorldSignal" });
    open();
  }

  function openEdit(c: EmailConnector) {
    setEditing(c);
    form.setValues({ name: c.name, provider: c.provider, host: c.host, port: c.port, security: c.security, username: c.username, secret: "", fromEmail: c.fromEmail, fromName: c.fromName });
    open();
  }

  async function submit(v: FormValues) {
    setBusy(true);
    try {
      const input: Record<string, unknown> = {
        name: v.name, provider: v.provider, host: v.host, port: v.port, security: v.security,
        username: v.username, fromEmail: v.fromEmail, fromName: v.fromName,
      };
      if (v.secret.trim()) input.secret = v.secret;
      const c = editing ? await api.updateEmailConnector(editing.id, input) : await api.createEmailConnector(input);
      notifications.show({
        message: c.status === "VALID" ? "Connector saved and verified" : `Connector saved, but the connection test failed (${c.status})${c.lastError ? `: ${c.lastError}` : ""}`,
        color: c.status === "VALID" ? "green" : "orange",
      });
      close(); connectors.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally { setBusy(false); }
  }

  async function act(fn: () => Promise<unknown>, msg: string) {
    try { await fn(); notifications.show({ message: msg, color: "green" }); connectors.reload(); }
    catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  async function test(c: EmailConnector) {
    try {
      const r = await api.testEmailConnector(c.id);
      notifications.show({ message: r.ok ? "Connection OK" : `Failed: ${r.error ?? r.status}`, color: r.ok ? "green" : "red" });
      connectors.reload();
    } catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  async function sendTest(c: EmailConnector) {
    const to = window.prompt("Send a test email to:");
    if (!to) return;
    try {
      const r = await api.sendTestEmail(c.id, to);
      notifications.show({ message: r.ok ? `Test email sent to ${to}` : `Failed: ${r.error}`, color: r.ok ? "green" : "red" });
    } catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  const active = connectors.data?.find((c) => c.isActive);

  return (
    <>
      <PageHeader title="Connectors" subtitle="Email (SMTP) connectors for delivering signals"
        actions={<Button leftSection={<IconPlus size={16} />} onClick={openCreate} data-testid="add-connector">Add connector</Button>} />
      <Stack gap="md">
        <Alert color={active ? "green" : "orange"} icon={<IconMail size={18} />}
          title={active ? `Active connector: ${active.name}` : "No active email connector"} data-testid="connector-status">
          {active
            ? <Text size="sm">Email subscriptions send through <b>{active.fromEmail}</b> via {active.host}. Set a different connector active to switch.</Text>
            : <Text size="sm">Add a connector and set it active, then create an EMAIL subscription. Gmail users: create an <b>App Password</b> — see the setup guide in <Anchor href="https://github.com/jigardafda/WorldSignal/blob/main/docs/EMAIL.md" target="_blank" rel="noreferrer">docs/EMAIL.md</Anchor>.</Text>}
        </Alert>

        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={connectors} empty={(r) => r.length === 0}>
            {(rows) => (
              <DataTable rows={rows} getKey={(c) => c.id}
                columns={[
                  { key: "name", header: "Name", render: (c) => c.name },
                  { key: "provider", header: "Provider", render: (c) => c.provider },
                  { key: "from", header: "From", render: (c) => c.fromEmail },
                  { key: "server", header: "Server", render: (c) => `${c.host}:${c.port}` },
                  { key: "status", header: "Status", render: (c) => <ValidationBadge status={c.status} /> },
                  { key: "active", header: "Active", render: (c) => (c.isActive ? <Badge color="blue">active</Badge> : c.enabled ? <Text c="dimmed" size="sm">—</Text> : <Badge color="gray">disabled</Badge>) },
                  { key: "lastTestedAt", header: "Last tested", render: (c) => fmtDate(c.lastTestedAt) },
                  {
                    key: "actions", header: "", render: (c: EmailConnector) => (
                      <Group gap="xs" wrap="nowrap">
                        {!c.isActive && <Button size="xs" variant="light" onClick={() => act(() => api.setActiveEmailConnector(c.id), "Activated")}>Set active</Button>}
                        <Button size="xs" variant="light" color="grape" onClick={() => test(c)}>Test</Button>
                        <Button size="xs" variant="light" color="teal" leftSection={<IconSend size={13} />} onClick={() => sendTest(c)}>Send test</Button>
                        <Button size="xs" variant="subtle" onClick={() => openEdit(c)}>Edit</Button>
                        <ConfirmButton label="Delete" message={`Delete connector "${c.name}"?`} confirmLabel="Delete"
                          onConfirm={() => api.deleteEmailConnector(c.id)} onDone={connectors.reload} />
                      </Group>
                    ),
                  },
                ]} />
            )}
          </AsyncBoundary>
          {connectors.data && connectors.data.length === 0 && (
            <Text c="dimmed" size="sm" mt="sm">No connectors yet. Add one to enable email delivery.</Text>
          )}
        </Paper>
      </Stack>

      <Modal opened={opened} onClose={close} title={editing ? "Edit connector" : "Add email connector"} centered size="lg">
        <form onSubmit={form.onSubmit(submit)}>
          <Stack>
            <Select label="Provider" data={(providers.data ?? []).map((p) => ({ value: p.code, label: p.label }))}
              value={form.values.provider} onChange={(v) => v && pickProvider(v)} allowDeselect={false} data-testid="conn-provider" />
            {preset?.help && <Alert color="blue" variant="light" py="xs"><Text size="xs">{preset.help}</Text></Alert>}
            <TextInput label="Name" placeholder="Team alerts" required {...form.getInputProps("name")} data-testid="conn-name" />
            <Group grow>
              <TextInput label="From address" placeholder="alerts@yourdomain.com" required {...form.getInputProps("fromEmail")} data-testid="conn-from" />
              <TextInput label="From name" {...form.getInputProps("fromName")} />
            </Group>
            <Group grow>
              <TextInput label="SMTP host" required {...form.getInputProps("host")} />
              <NumberInput label="Port" min={1} max={65535} {...form.getInputProps("port")} />
            </Group>
            <Select label="Security" data={SECURITY} {...form.getInputProps("security")} allowDeselect={false} />
            <TextInput label="Username" description={preset?.usernameHint} {...form.getInputProps("username")} data-testid="conn-username" />
            <PasswordInput label="Password / API key" description={preset?.secretHint}
              placeholder={editing ? "Leave blank to keep the stored secret" : "••••••••"}
              {...form.getInputProps("secret")} data-testid="conn-secret" />
            <Text size="xs" c="dimmed">The secret is encrypted at rest and validated by opening a connection on save. Stored secrets are never shown again.</Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>Cancel</Button>
              <Button type="submit" loading={busy} data-testid="conn-submit">{editing ? "Save" : "Add & verify"}</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </>
  );
}
