import { Group, Pagination, Paper, Stack, TextInput } from "@mantine/core";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type ArticleRow, type Page } from "../lib/api";
import { usePagedList } from "../lib/usePagedList";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { fmtDate } from "../lib/format";

export function Articles() {
  const navigate = useNavigate();
  const [pending, setPending] = useState("");
  const [search, setSearch] = useState("");
  const { state, page, setPage, totalPages } = usePagedList<ArticleRow>(
    (limit, offset) => api.articles({ search: search || undefined, limit, offset }),
    [search],
  );

  return (
    <>
      <PageHeader title="Articles" subtitle={state.data ? `${state.data.total} normalized articles` : "Normalized articles"} />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <TextInput placeholder="Search title…" value={pending}
            onChange={(e) => setPending(e.currentTarget.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { setSearch(pending); setPage(1); } }}
            data-testid="article-search" />
        </Paper>
        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={state} empty={(p: Page<ArticleRow>) => p.items.length === 0}>
            {(p) => (
              <DataTable
                rows={p.items}
                getKey={(r) => r.id}
                onRowClick={(r) => navigate(`/articles/${r.id}`)}
                columns={[
                  { key: "title", header: "Title", render: (r) => r.title },
                  { key: "sourceName", header: "Source", render: (r) => r.sourceName },
                  { key: "signalCount", header: "Signals", render: (r) => r.signalCount },
                  { key: "publishedAt", header: "Published", render: (r) => fmtDate(r.publishedAt) },
                  { key: "fetchedAt", header: "Fetched", render: (r) => fmtDate(r.fetchedAt) },
                ]}
              />
            )}
          </AsyncBoundary>
          {totalPages > 1 && <Group justify="center" mt="md"><Pagination total={totalPages} value={page} onChange={setPage} /></Group>}
        </Paper>
      </Stack>
    </>
  );
}
