import { Group, Pagination, Paper, Select, Stack } from "@mantine/core";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Page, type RawItemRow } from "../lib/api";
import { usePagedList } from "../lib/usePagedList";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

const STATUSES = ["PENDING", "PARSED", "FAILED", "DUPLICATE"];

export function RawItems() {
  const navigate = useNavigate();
  const [status, setStatus] = useState<string | null>(null);
  const { state, page, setPage, totalPages } = usePagedList<RawItemRow>(
    (limit, offset) => api.rawItems({ status: status || undefined, limit, offset }),
    [status],
  );
  return (
    <>
      <PageHeader title="Raw Items" subtitle={state.data ? `${state.data.total} raw evidence items` : "Raw ingested evidence"} />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <Select placeholder="Status" clearable data={STATUSES} value={status} onChange={(v) => { setStatus(v); setPage(1); }} data-testid="raw-status" w={220} />
        </Paper>
        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={state} empty={(p: Page<RawItemRow>) => p.items.length === 0}>
            {(p) => (
              <DataTable rows={p.items} getKey={(r) => r.id} onRowClick={(r) => navigate(`/raw-items/${r.id}`)}
                columns={[
                  { key: "rawTitle", header: "Title", render: (r) => r.rawTitle ?? "—" },
                  { key: "sourceName", header: "Source", render: (r) => r.sourceName },
                  { key: "status", header: "Status", render: (r) => <StatusBadge status={r.status} /> },
                  { key: "fetchedAt", header: "Fetched", render: (r) => fmtDate(r.fetchedAt) },
                ]} />
            )}
          </AsyncBoundary>
          {totalPages > 1 && <Group justify="center" mt="md"><Pagination total={totalPages} value={page} onChange={setPage} /></Group>}
        </Paper>
      </Stack>
    </>
  );
}
