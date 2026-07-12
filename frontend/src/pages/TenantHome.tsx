import { Button, Group, Paper, SimpleGrid, Stack, Text, Title } from "@mantine/core";
import { IconActivity, IconArrowRight, IconKey, IconRocket, IconSparkles, IconWorld } from "@tabler/icons-react";
import { useNavigate } from "react-router-dom";
import { api, type ApiKey, type Signal } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { StatCard } from "../components/StatCard";
import { DataTable } from "../components/DataTable";
import { SeverityBadge, ConfidenceBar, SignalIntel } from "../components/badges";
import { fmtDate } from "../lib/format";

// TenantHome is the customer console landing page: workspace + plan, API usage,
// quick actions, and the latest relevant signals — customer-facing metrics, not
// the operator pipeline view.
export function TenantHome() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const keys = useAsync<ApiKey[]>(() => api.myApiKeys(), []);
  const recent = useAsync<Signal[]>(() => api.signals({}, 8, 0), []);

  const account = user?.account;
  const keyCount = keys.data?.length ?? 0;
  const requests = (keys.data ?? []).reduce((n, k) => n + (k.requestCount ?? 0), 0);

  return (
    <>
      <PageHeader title="Dashboard" subtitle={account ? `${account.name} · your WorldSignal workspace` : "Your WorldSignal workspace"} />
      <Stack gap="lg">
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }}>
          <StatCard label="Plan" value={account?.plan ?? "—"} icon={<IconRocket size={20} />} color="teal" />
          <StatCard label="API keys" value={keyCount} icon={<IconKey size={20} />} color="blue" />
          <StatCard label="API requests" value={requests.toLocaleString()} icon={<IconWorld size={20} />} color="grape" />
          <StatCard label="Status" value={account?.status ?? "—"} icon={<IconActivity size={20} />} color="green" />
        </SimpleGrid>

        <Paper withBorder p="md" radius="md">
          <Title order={5} mb="xs">Quick actions</Title>
          <Group>
            <Button variant="light" leftSection={<IconKey size={16} />} rightSection={<IconArrowRight size={14} />}
              onClick={() => navigate("/my-api-keys")} data-testid="qa-keys">Create an API key</Button>
            <Button variant="light" leftSection={<IconSparkles size={16} />} rightSection={<IconArrowRight size={14} />}
              onClick={() => navigate("/my-subscriptions")} data-testid="qa-personalize">Set up delivery</Button>
            <Button variant="light" leftSection={<IconActivity size={16} />} rightSection={<IconArrowRight size={14} />}
              onClick={() => navigate("/signals")} data-testid="qa-signals">Browse signals</Button>
          </Group>
        </Paper>

        <Paper withBorder p="md" radius="md">
          <Group justify="space-between" mb="sm">
            <Title order={4}>Latest signals</Title>
            <Button size="xs" variant="subtle" rightSection={<IconArrowRight size={14} />} onClick={() => navigate("/signals")}>View all</Button>
          </Group>
          <AsyncBoundary state={recent} empty={(rows) => rows.length === 0}>
            {(rows) => (
              <DataTable
                rows={rows}
                getKey={(r) => r.id}
                onRowClick={(r) => navigate(`/signals/${r.id}`)}
                columns={[
                  { key: "severity", header: "Severity", render: (r) => <SeverityBadge severity={r.severity} /> },
                  { key: "title", header: "Title", render: (r) => (
                    <div>
                      <Text fw={500} size="sm">{r.title}</Text>
                      <SignalIntel signal={r} />
                    </div>
                  ) },
                  { key: "confidence", header: "Confidence", render: (r) => <ConfidenceBar value={r.confidence} /> },
                  { key: "lastSeenAt", header: "Last seen", render: (r) => fmtDate(r.lastSeenAt) },
                ]}
              />
            )}
          </AsyncBoundary>
        </Paper>
      </Stack>
    </>
  );
}
