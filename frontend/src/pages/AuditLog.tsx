import { Badge, Code, Group, Paper, Pagination, Stack, Text, TextInput } from "@mantine/core";
import { useState } from "react";
import { api, type AuditLog as AuditLogRow, type AuditFilter } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { fmtDate } from "../lib/format";

const PAGE_SIZE = 50;

function metaText(m: unknown): string {
  if (m == null) return "—";
  try {
    const s = typeof m === "string" ? m : JSON.stringify(m);
    return s === "{}" || s === "null" ? "—" : s;
  } catch {
    return "—";
  }
}

export function AuditLog() {
  const [pending, setPending] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const filter: AuditFilter = {};
  if (search) filter.search = search;

  const list = useAsync(() => api.auditLogs(filter, PAGE_SIZE, (page - 1) * PAGE_SIZE), [search, page]);
  const total = list.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <>
      <PageHeader title="Audit Log" subtitle={`${total.toLocaleString()} recorded actions`} />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <TextInput
            placeholder="Search action, actor or target…"
            value={pending}
            onChange={(e) => setPending(e.currentTarget.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { setSearch(pending); setPage(1); } }}
            data-testid="audit-search"
          />
        </Paper>
        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={list} empty={(p) => p.items.length === 0}>
            {(p) => (
              <DataTable<AuditLogRow>
                rows={p.items}
                getKey={(r) => r.id}
                columns={[
                  { key: "createdAt", header: "Time", render: (r) => fmtDate(r.createdAt) },
                  { key: "actor", header: "Actor", render: (r) => r.actorEmail ?? "—" },
                  { key: "role", header: "Role", render: (r) => (r.actorRole ? <Badge size="sm" variant="light">{r.actorRole}</Badge> : "—") },
                  { key: "action", header: "Action", render: (r) => <Code>{r.action}</Code> },
                  { key: "target", header: "Target", render: (r) => (r.targetType ? `${r.targetType}${r.targetId ? `:${r.targetId}` : ""}` : "—") },
                  { key: "meta", header: "Details", render: (r) => <Text size="xs" c="dimmed" lineClamp={1}>{metaText(r.metadata)}</Text> },
                ]}
              />
            )}
          </AsyncBoundary>
          {totalPages > 1 && (
            <Group justify="center" mt="md">
              <Pagination total={totalPages} value={page} onChange={setPage} />
            </Group>
          )}
        </Paper>
      </Stack>
    </>
  );
}
