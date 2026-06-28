import { Button, Group, Pagination, Paper, Select, Stack } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type DeliveryRow, type Page } from "../lib/api";
import { usePagedList } from "../lib/usePagedList";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

const STATUSES = ["PENDING", "SENT", "FAILED", "RETRYING", "DEAD_LETTERED"];

export function Deliveries() {
  const navigate = useNavigate();
  const { hasPerm } = useAuth();
  const canRetry = hasPerm("deliveries:retry");
  const [status, setStatus] = useState<string | null>(null);
  const { state, page, setPage, totalPages } = usePagedList<DeliveryRow>(
    (limit, offset) => api.deliveries({ status: status || undefined, limit, offset }),
    [status],
  );

  async function retry(id: string) {
    try {
      await api.retryDelivery(id);
      notifications.show({ message: "Delivery re-queued", color: "green" });
      state.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    }
  }

  return (
    <>
      <PageHeader title="Deliveries" subtitle={state.data ? `${state.data.total} delivery events` : "Webhook & polling deliveries"} />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <Select placeholder="Status" clearable data={STATUSES} value={status} onChange={(v) => { setStatus(v); setPage(1); }} data-testid="delivery-status" w={220} />
        </Paper>
        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={state} empty={(p: Page<DeliveryRow>) => p.items.length === 0}>
            {(p) => (
              <DataTable rows={p.items} getKey={(r) => r.id} onRowClick={(r) => navigate(`/deliveries/${r.id}`)}
                columns={[
                  { key: "signalTitle", header: "Signal", render: (r) => r.signalTitle },
                  { key: "subscriptionName", header: "Subscription", render: (r) => r.subscriptionName },
                  { key: "channel", header: "Channel", render: (r) => r.channel },
                  { key: "status", header: "Status", render: (r) => <StatusBadge status={r.status} /> },
                  { key: "attempts", header: "Attempts", render: (r) => r.attempts },
                  { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                  ...(canRetry ? [{
                    key: "actions", header: "", render: (r: DeliveryRow) => (
                      <Group onClick={(e) => e.stopPropagation()}>
                        <Button size="xs" variant="light" disabled={r.status === "SENT"} onClick={() => retry(r.id)}>Retry</Button>
                      </Group>
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
