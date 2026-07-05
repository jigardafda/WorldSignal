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
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

export function ApiKeys() {
  const keys = useAsync<ApiKey[]>(() => api.apiKeys(), []);
  const scopes = useAsync<string[]>(() => api.apiScopes(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  // The raw key is shown exactly once, right after creation.
  const [created, setCreated] = useState<string | null>(null);

  const form = useForm({
    initialValues: { name: "", scopes: [] as string[], rateLimitPerMin: 120, expiresAt: "" },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      scopes: (v) => (v.length ? null : "Select at least one scope"),
    },
  });

  async function create(v: typeof form.values) {
    setBusy(true);
    try {
      const input: { name: string; scopes: string[]; rateLimitPerMin?: number; expiresAt?: string } = {
        name: v.name, scopes: v.scopes, rateLimitPerMin: v.rateLimitPerMin,
      };
      if (v.expiresAt.trim()) input.expiresAt = new Date(v.expiresAt).toISOString();
      const k = await api.createApiKey(input);
      close(); form.reset(); keys.reload();
      setCreated(k.key ?? null); // reveal once
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally { setBusy(false); }
  }

  async function toggle(k: ApiKey) {
    try { await api.setApiKeyEnabled(k.id, !k.enabled); keys.reload(); }
    catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  return (
    <>
      <PageHeader title="API Keys" subtitle="Credentials for the public REST API (/v1/*)"
        actions={<Button leftSection={<IconPlus size={16} />} onClick={open} data-testid="add-key">Create API key</Button>} />
      <Stack gap="md">
        <Alert color="blue" icon={<IconKey size={18} />} title="How it works">
          <Text size="sm">
            Every <Code>/v1/*</Code> endpoint requires a key sent as <Code>Authorization: Bearer &lt;key&gt;</Code> or
            {" "}<Code>X-API-Key: &lt;key&gt;</Code>. Each key is limited to the scopes you grant and a per-minute rate limit.
            Keys are stored hashed — the secret is shown once, at creation.
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
                  { key: "enabled", header: "Status", render: (k) => <StatusBadge status={k.enabled ? "ACTIVE" : "SUSPENDED"} /> },
                  { key: "lastUsedAt", header: "Last used", render: (k) => (k.lastUsedAt ? fmtDate(k.lastUsedAt) : "never") },
                  { key: "requestCount", header: "Requests", render: (k) => k.requestCount },
                  {
                    key: "actions", header: "", render: (k: ApiKey) => (
                      <Group gap="xs" wrap="nowrap">
                        <Button size="xs" variant="light" color={k.enabled ? "orange" : "green"} onClick={() => toggle(k)}>
                          {k.enabled ? "Disable" : "Enable"}
                        </Button>
                        <ConfirmButton label="Delete" message={`Delete API key "${k.name}"? Any client using it will stop working.`} confirmLabel="Delete"
                          onConfirm={() => api.deleteApiKey(k.id)} onDone={keys.reload} />
                      </Group>
                    ),
                  },
                ]} />
            )}
          </AsyncBoundary>
          {keys.data && keys.data.length === 0 && (
            <Text c="dimmed" size="sm" mt="sm">No API keys yet. Create one to grant programmatic access to the REST API.</Text>
          )}
        </Paper>
      </Stack>

      <Modal opened={opened} onClose={close} title="Create API key" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" placeholder="Analytics pipeline" required {...form.getInputProps("name")} data-testid="key-name" />
            <MultiSelect label="Scopes" placeholder="Pick scopes" required searchable
              data={scopes.data ?? []} {...form.getInputProps("scopes")} data-testid="key-scopes" />
            <NumberInput label="Rate limit (requests / minute)" min={1} max={100000} {...form.getInputProps("rateLimitPerMin")} />
            <TextInput label="Expires (optional)" type="date" {...form.getInputProps("expiresAt")} data-testid="key-expires" />
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
