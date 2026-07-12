import {
  Alert, Badge, Button, Code, CopyButton, Group, Modal, MultiSelect, NumberInput,
  Paper, Stack, Text, TextInput,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconCheck, IconCopy, IconKey, IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { api, type ApiKey } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { fmtDate } from "../lib/format";

// MyApiKeys is the tenant (customer console) self-service view: a tenant manages
// only its own API keys, scoped to its account and limited to read-only scopes.
export function MyApiKeys() {
  const keys = useAsync<ApiKey[]>(() => api.myApiKeys(), []);
  const scopes = useAsync<string[]>(() => api.tenantApiScopes(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<string | null>(null);

  const form = useForm({
    initialValues: { name: "", scopes: [] as string[], rateLimitPerMin: 120 },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      scopes: (v) => (v.length ? null : "Select at least one scope"),
    },
  });

  async function create(v: typeof form.values) {
    setBusy(true);
    try {
      const k = await api.createMyApiKey({ name: v.name, scopes: v.scopes, rateLimitPerMin: v.rateLimitPerMin });
      close(); form.reset(); keys.reload();
      setCreated(k.key ?? null);
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally { setBusy(false); }
  }

  return (
    <>
      <PageHeader title="API Keys" subtitle="Programmatic access to WorldSignal, scoped to your account"
        actions={<Button leftSection={<IconPlus size={16} />} onClick={open} data-testid="add-key">Create API key</Button>} />
      <Stack gap="md">
        <Alert color="blue" icon={<IconKey size={18} />} title="How it works">
          <Text size="sm">
            Send your key as <Code>Authorization: Bearer &lt;key&gt;</Code> or <Code>X-API-Key: &lt;key&gt;</Code> to any
            {" "}<Code>/v1/*</Code> endpoint. Keys are read-only and stored hashed — the secret is shown once, at creation.
          </Text>
        </Alert>
        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={keys} empty={(r) => r.length === 0}>
            {(rows) => (
              <DataTable rows={rows} getKey={(k) => k.id}
                columns={[
                  { key: "name", header: "Name", render: (k) => k.name },
                  { key: "prefix", header: "Key", render: (k) => <Text ff="monospace" size="sm">{k.keyPrefix}…</Text> },
                  { key: "scopes", header: "Scopes", render: (k) => (
                    <Group gap={4}>{k.scopes.map((s) => <Badge key={s} size="sm" variant="light">{s}</Badge>)}</Group>
                  ) },
                  { key: "rate", header: "Rate/min", render: (k) => k.rateLimitPerMin },
                  { key: "lastUsedAt", header: "Last used", render: (k) => (k.lastUsedAt ? fmtDate(k.lastUsedAt) : "never") },
                  { key: "requestCount", header: "Requests", render: (k) => k.requestCount },
                  {
                    key: "actions", header: "", render: (k: ApiKey) => (
                      <ConfirmButton label="Revoke" message={`Revoke API key "${k.name}"? Any client using it will stop working.`} confirmLabel="Revoke"
                        onConfirm={() => api.revokeMyApiKey(k.id)} onDone={keys.reload} />
                    ),
                  },
                ]} />
            )}
          </AsyncBoundary>
        </Paper>
      </Stack>

      <Modal opened={opened} onClose={close} title="Create API key" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" placeholder="My integration" required {...form.getInputProps("name")} data-testid="key-name" />
            <MultiSelect label="Scopes" placeholder="Pick scopes" required searchable
              data={scopes.data ?? []} {...form.getInputProps("scopes")} data-testid="key-scopes" />
            <NumberInput label="Rate limit (requests / minute)" min={1} max={100000} {...form.getInputProps("rateLimitPerMin")} />
            <Text size="xs" c="dimmed">The secret is shown only once, right after creation. Store it somewhere safe.</Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>Cancel</Button>
              <Button type="submit" loading={busy} data-testid="key-submit">Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      <Modal opened={created !== null} onClose={() => setCreated(null)} title="Your new API key" centered data-testid="key-reveal">
        <Stack>
          <Alert color="yellow">This is the only time the key is shown. Copy it now — it can't be retrieved later.</Alert>
          <Group gap="xs" wrap="nowrap">
            <Code style={{ flex: 1, wordBreak: "break-all" }} data-testid="key-value">{created}</Code>
            <CopyButton value={created ?? ""}>
              {({ copied, copy }) => (
                <Button size="xs" variant="light" color={copied ? "teal" : "blue"}
                  leftSection={copied ? <IconCheck size={14} /> : <IconCopy size={14} />} onClick={copy}>
                  {copied ? "Copied" : "Copy"}
                </Button>
              )}
            </CopyButton>
          </Group>
          <Group justify="flex-end"><Button onClick={() => setCreated(null)}>Done</Button></Group>
        </Stack>
      </Modal>
    </>
  );
}
