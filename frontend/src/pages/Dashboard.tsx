import { useState } from "react";
import { SegmentedControl, SimpleGrid, Stack, Paper, Text, Title, Group } from "@mantine/core";
import { IconActivity, IconArticle, IconBroadcast, IconSend, IconClock } from "@tabler/icons-react";
import { useNavigate } from "react-router-dom";
import { api, type Signal, type Stats } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { StatCard } from "../components/StatCard";
import { DataTable } from "../components/DataTable";
import { SeverityBadge, ConfidenceBar, SignalIntel } from "../components/badges";
import { fmtDate } from "../lib/format";
import { LiveDashboard } from "./LiveDashboard";

export function Dashboard() {
  const navigate = useNavigate();
  const [mode, setMode] = useState<string>("dashboard");
  const stats = useAsync<Stats>(() => api.stats(), []);
  const recent = useAsync<Signal[]>(() => api.signals({}, 8, 0), []);

  return (
    <>
      <PageHeader
        title="Dashboard"
        subtitle="Live overview of the WorldSignal pipeline"
        actions={
          <SegmentedControl
            value={mode}
            onChange={setMode}
            data-testid="dashboard-mode"
            data={[
              { value: "dashboard", label: "Dashboard" },
              { value: "live", label: "Live" },
            ]}
          />
        }
      />
      {mode === "live" ? (
        <LiveDashboard />
      ) : (
      <Stack gap="lg">
        <AsyncBoundary state={stats}>
          {(s) => (
            <SimpleGrid cols={{ base: 1, sm: 2, lg: 5 }}>
              <StatCard label="Sources" value={s.sources} icon={<IconBroadcast size={20} />} color="blue" />
              <StatCard label="Articles" value={s.articles} icon={<IconArticle size={20} />} color="grape" />
              <StatCard label="Signals" value={s.signals} icon={<IconActivity size={20} />} color="teal" />
              <StatCard label="Deliveries sent" value={s.deliveriesSent} icon={<IconSend size={20} />} color="green" />
              <StatCard label="Deliveries pending" value={s.deliveriesPending} icon={<IconClock size={20} />} color="orange" />
            </SimpleGrid>
          )}
        </AsyncBoundary>

        <Paper withBorder p="md" radius="md">
          <Group justify="space-between" mb="sm">
            <Title order={4}>Latest signals</Title>
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
                  { key: "sourceCount", header: "Sources", render: (r) => r.sourceCount },
                  { key: "confidence", header: "Confidence", render: (r) => <ConfidenceBar value={r.confidence} /> },
                  { key: "lastSeenAt", header: "Last seen", render: (r) => fmtDate(r.lastSeenAt) },
                ]}
              />
            )}
          </AsyncBoundary>
        </Paper>
      </Stack>
      )}
    </>
  );
}
