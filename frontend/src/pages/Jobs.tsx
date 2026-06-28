import { Button, Code, Group, Pagination, Paper, Select, SimpleGrid, Stack } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useState } from "react";
import { api, type Bucket, type Job, type Page } from "../lib/api";
import { usePagedList } from "../lib/usePagedList";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { StatCard } from "../components/StatCard";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

const STATES = ["created", "active", "completed", "failed"];

export function Jobs() {
  const { hasPerm } = useAuth();
  const canManage = hasPerm("jobs:manage");
  const [state, setState] = useState<string | null>(null);
  const counts = useAsync<Bucket[]>(() => api.jobCounts(), []);
  const { state: list, page, setPage, totalPages } = usePagedList<Job>(
    (limit, offset) => api.jobs({ state: state || undefined, limit, offset }),
    [state],
  );

  async function retry(id: string) {
    try {
      await api.retryJob(id);
      notifications.show({ message: "Job re-queued", color: "green" });
      list.reload();
      counts.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    }
  }

  return (
    <>
      <PageHeader title="Jobs" subtitle="Background queue (fetch → normalize → enrich → deliver)" />
      <Stack gap="md">
        <AsyncBoundary state={counts}>
          {(cs) => (
            <SimpleGrid cols={{ base: 2, sm: 4 }}>
              {cs.length === 0
                ? <StatCard label="Jobs" value={0} />
                : cs.map((c) => <StatCard key={c.key} label={c.key} value={c.count} />)}
            </SimpleGrid>
          )}
        </AsyncBoundary>
        <Paper withBorder p="sm" radius="md">
          <Select placeholder="State" clearable data={STATES} value={state} onChange={(v) => { setState(v); setPage(1); }} data-testid="job-state" w={220} />
        </Paper>
        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={list} empty={(p: Page<Job>) => p.items.length === 0}>
            {(p) => (
              <DataTable rows={p.items} getKey={(r) => r.id}
                columns={[
                  { key: "queue", header: "Queue", render: (r) => <Code>{r.queue}</Code> },
                  { key: "state", header: "State", render: (r) => <StatusBadge status={r.state} /> },
                  { key: "retryCount", header: "Retries", render: (r) => `${r.retryCount}/${r.retryLimit}` },
                  { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                  { key: "lastError", header: "Last error", render: (r) => r.lastError ?? "—" },
                  ...(canManage ? [{
                    key: "actions", header: "", render: (r: Job) => (
                      <Button size="xs" variant="light" onClick={() => retry(r.id)}>Retry</Button>
                    ),
                  }] : []),
                ]} />
            )}
          </AsyncBoundary>
          {totalPages > 1 && <Group justify="center" mt="md"><Pagination total={totalPages} value={page} onChange={setPage} /></Group>}
        </Paper>
      </Stack>
    </>
  );
}
