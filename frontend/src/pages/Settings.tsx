import { Alert, Badge, Button, Group, Modal, Paper, PasswordInput, Select, Stack, Text, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconCheck, IconKey, IconPlus, IconX } from "@tabler/icons-react";
import { useState } from "react";
import { api, type LLMKey, type LLMStatus } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { ValidationBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

export function Settings() {
  const status = useAsync<LLMStatus>(() => api.llmStatus(), []);
  const keys = useAsync<LLMKey[]>(() => api.llmKeys(), []);
  // Model options are fetched live from the provider (via the effective key) —
  // never hardcoded, so the list always reflects what the account can use.
  const models = useAsync<string[]>(() => api.llmModels(), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);

  const form = useForm({
    initialValues: { label: "", key: "", model: "" },
    validate: {
      label: (v) => (v.trim() ? null : "Label is required"),
      key: (v) => (v.trim().length >= 8 ? null : "Enter a valid API key"),
    },
  });

  function reload() {
    status.reload();
    keys.reload();
  }

  async function create(values: typeof form.values) {
    setBusy(true);
    try {
      const k = await api.createLLMKey({ provider: "OPENAI", label: values.label, key: values.key, model: values.model || undefined });
      notifications.show({
        message: k.status === "VALID" ? "Key added and validated against OpenAI" : `Key added but validation failed (${k.status})`,
        color: k.status === "VALID" ? "green" : "orange",
      });
      close();
      form.reset();
      reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally {
      setBusy(false);
    }
  }

  async function act(fn: () => Promise<unknown>, msg: string) {
    try {
      await fn();
      notifications.show({ message: msg, color: "green" });
      reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    }
  }

  async function test(id: string) {
    try {
      const r = await api.testLLMKey(id);
      notifications.show({ message: r.ok ? "Key is valid" : `Invalid: ${r.error ?? r.status}`, color: r.ok ? "green" : "red" });
      reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    }
  }

  return (
    <>
      <PageHeader
        title="Settings"
        subtitle="LLM provider keys for article enrichment"
        actions={<Button leftSection={<IconPlus size={16} />} onClick={open}>Add OpenAI key</Button>}
      />
      <Stack gap="md">
        <AsyncBoundary state={status}>
          {(st) => (
            <Alert
              color={st.enabled ? "green" : "orange"}
              icon={st.enabled ? <IconCheck size={18} /> : <IconX size={18} />}
              title={st.enabled ? "LLM enrichment is active" : "LLM enrichment is disabled"}
              data-testid="llm-status"
            >
              <Group gap="xl">
                <Text size="sm"><b>Provider:</b> {st.provider}</Text>
                <Text size="sm"><b>Active key source:</b> {st.source === "DB" ? `Admin key${st.activeLabel ? ` (${st.activeLabel})` : ""}` : st.source === "ENV" ? "System key (env)" : "None"}</Text>
                <Text size="sm"><b>Model:</b> {st.model || "—"}</Text>
                <Text size="sm"><b>System key present:</b> {st.hasSystemKey ? "yes" : "no"}</Text>
              </Group>
              {!st.enabled && <Text size="sm" mt="xs" c="dimmed">Add a key below — enrichment falls back to the built-in heuristic until one is active.</Text>}
            </Alert>
          )}
        </AsyncBoundary>

        <Paper withBorder p="md" radius="md">
          <Group mb="sm" gap="xs"><IconKey size={18} /><Text fw={700}>Admin-managed keys</Text></Group>
          <AsyncBoundary state={keys} empty={(rows) => rows.length === 0}>
            {(rows) => (
              <DataTable
                rows={rows}
                getKey={(k) => k.id}
                columns={[
                  { key: "label", header: "Label", render: (k) => k.label },
                  { key: "provider", header: "Provider", render: (k) => k.provider },
                  { key: "key", header: "Key", render: (k) => <Text ff="monospace" size="sm">••••{k.keyLast4}</Text> },
                  { key: "model", header: "Model", render: (k) => k.model ?? "—" },
                  { key: "status", header: "Status", render: (k) => <ValidationBadge status={k.status} /> },
                  { key: "active", header: "Active", render: (k) => (k.isActive ? <Badge color="blue">active</Badge> : <Text c="dimmed" size="sm">—</Text>) },
                  { key: "lastTestedAt", header: "Last tested", render: (k) => fmtDate(k.lastTestedAt) },
                  {
                    key: "actions", header: "", render: (k: LLMKey) => (
                      <Group gap="xs" wrap="nowrap">
                        {!k.isActive && <Button size="xs" variant="light" onClick={() => act(() => api.setActiveLLMKey(k.id), "Activated")}>Set active</Button>}
                        <Button size="xs" variant="light" color="grape" onClick={() => test(k.id)}>Test</Button>
                        <ConfirmButton label="Delete" message={`Delete key "${k.label}"?`} confirmLabel="Delete"
                          onConfirm={() => api.deleteLLMKey(k.id)} onDone={reload} />
                      </Group>
                    ),
                  },
                ]}
              />
            )}
          </AsyncBoundary>
          {keys.data && keys.data.length === 0 && (
            <Text c="dimmed" size="sm" mt="sm">No admin keys yet. The system key from the environment is used when present.</Text>
          )}
        </Paper>
      </Stack>

      <Modal opened={opened} onClose={close} title="Add OpenAI key" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Label" placeholder="Production OpenAI" required {...form.getInputProps("label")} data-testid="llm-label" />
            <PasswordInput label="API key" placeholder="sk-…" required {...form.getInputProps("key")} data-testid="llm-key" />
            <Select
              label="Model (optional)"
              placeholder={models.loading ? "Loading models…" : (models.data && models.data.length ? "Inherit system default" : "No models — add a valid key first")}
              data={models.data ?? []}
              disabled={!models.data || models.data.length === 0}
              searchable
              clearable
              nothingFoundMessage="No matching model"
              {...form.getInputProps("model")}
              data-testid="llm-model"
            />
            <Text size="xs" c="dimmed">The key is encrypted at rest and validated against OpenAI on save. Stored keys are never shown again — only the last 4 characters.</Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>Cancel</Button>
              <Button type="submit" loading={busy}>Add &amp; validate</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </>
  );
}
