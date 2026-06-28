import { Group, Pagination, Paper, Select, Stack, TextInput } from "@mantine/core";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Signal } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfidenceBar, SeverityBadge, StatusBadge } from "../components/badges";
import { useCountries, countryDisplay } from "../lib/countries";
import { fmtDate } from "../lib/format";

const PAGE_SIZE = 25;
const STATUSES = ["UNVERIFIED", "DEVELOPING", "CONFIRMED", "DISPUTED", "CORRECTED", "RETRACTED", "RESOLVED"];

export function Signals() {
  const navigate = useNavigate();
  const { byCode } = useCountries();
  const [search, setSearch] = useState("");
  const [pendingSearch, setPendingSearch] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [minConf, setMinConf] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const filter: Record<string, unknown> = {};
  if (search) filter.search = search;
  if (status) filter.status = status;
  if (minConf) filter.minConfidence = Number(minConf);

  const list = useAsync<Signal[]>(() => api.signals(filter, PAGE_SIZE, (page - 1) * PAGE_SIZE), [search, status, minConf, page]);
  const count = useAsync<number>(() => api.signalCount(filter), [search, status, minConf]);
  const totalPages = Math.max(1, Math.ceil((count.data ?? 0) / PAGE_SIZE));

  function applySearch() {
    setSearch(pendingSearch);
    setPage(1);
  }

  return (
    <>
      <PageHeader title="Signals" subtitle={count.data != null ? `${count.data} signals` : "Browse canonical events"} />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <Group>
            <TextInput
              placeholder="Search title or summary…"
              value={pendingSearch}
              onChange={(e) => setPendingSearch(e.currentTarget.value)}
              onKeyDown={(e) => e.key === "Enter" && applySearch()}
              data-testid="signal-search"
              flex={1}
            />
            <Select placeholder="Status" clearable data={STATUSES} value={status}
              onChange={(v) => { setStatus(v); setPage(1); }} data-testid="signal-status" />
            <Select placeholder="Min confidence" clearable
              data={[{ value: "0.5", label: "≥ 50%" }, { value: "0.7", label: "≥ 70%" }, { value: "0.85", label: "≥ 85%" }]}
              value={minConf} onChange={(v) => { setMinConf(v); setPage(1); }} data-testid="signal-minconf" />
          </Group>
        </Paper>

        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={list} empty={(rows) => rows.length === 0}>
            {(rows) => (
              <DataTable
                rows={rows}
                getKey={(r) => r.id}
                onRowClick={(r) => navigate(`/signals/${r.id}`)}
                columns={[
                  { key: "severity", header: "Severity", render: (r) => <SeverityBadge severity={r.severity} /> },
                  { key: "title", header: "Title", render: (r) => r.title },
                  { key: "status", header: "Status", render: (r) => <StatusBadge status={r.status} /> },
                  { key: "country", header: "Country", render: (r) => countryDisplay(r.country, byCode) },
                  { key: "sourceCount", header: "Sources", render: (r) => r.sourceCount },
                  { key: "confidence", header: "Confidence", render: (r) => <ConfidenceBar value={r.confidence} /> },
                  { key: "lastSeenAt", header: "Last seen", render: (r) => fmtDate(r.lastSeenAt) },
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
