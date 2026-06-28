import { Paper, SimpleGrid, Stack, Text, Title } from "@mantine/core";
import { BarChart, DonutChart, AreaChart } from "@mantine/charts";
import { api, type Analytics as AnalyticsData } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { StatCard } from "../components/StatCard";
import { DataTable } from "../components/DataTable";

const DONUT = ["blue", "teal", "grape", "orange", "red", "cyan", "lime", "violet"];

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Paper withBorder p="md" radius="md">
      <Title order={5} mb="sm">{title}</Title>
      {children}
    </Paper>
  );
}

export function Analytics() {
  const state = useAsync<AnalyticsData>(() => api.analytics(), []);
  return (
    <>
      <PageHeader title="Analytics" subtitle="Signal, delivery and ingestion metrics" />
      <AsyncBoundary state={state}>
        {(a) => (
          <Stack gap="lg">
            <SimpleGrid cols={{ base: 2, sm: 3, lg: 6 }}>
              <StatCard label="Deliveries" value={a.deliveryStats.total} />
              <StatCard label="Sent" value={a.deliveryStats.sent} color="green" />
              <StatCard label="Failed" value={a.deliveryStats.failed} color="red" />
              <StatCard label="Dead-lettered" value={a.deliveryStats.deadLettered} color="dark" />
              <StatCard label="Raw items" value={a.ingestionStats.rawItems} color="grape" />
              <StatCard label="Articles" value={a.ingestionStats.articles} color="teal" />
            </SimpleGrid>

            <Panel title="Signals over time (30 days)">
              {a.signalsOverTime.length === 0 ? <Text c="dimmed" size="sm">No data yet.</Text> : (
                <AreaChart h={240} data={a.signalsOverTime} dataKey="key"
                  series={[{ name: "count", color: "blue.6", label: "Signals" }]} curveType="monotone" withDots={false} />
              )}
            </Panel>

            <SimpleGrid cols={{ base: 1, md: 2 }}>
              <Panel title="By severity">
                {a.signalsBySeverity.length === 0 ? <Text c="dimmed" size="sm">No data.</Text> : (
                  <DonutChart h={220} withLabels
                    data={a.signalsBySeverity.map((b, i) => ({ name: b.key, value: b.count, color: DONUT[i % DONUT.length] }))} />
                )}
              </Panel>
              <Panel title="By status">
                {a.signalsByStatus.length === 0 ? <Text c="dimmed" size="sm">No data.</Text> : (
                  <BarChart h={220} data={a.signalsByStatus} dataKey="key" series={[{ name: "count", color: "teal.6" }]} />
                )}
              </Panel>
              <Panel title="Top event types">
                {a.signalsByEventType.length === 0 ? <Text c="dimmed" size="sm">No data.</Text> : (
                  <BarChart h={240} data={a.signalsByEventType} dataKey="key" orientation="vertical" series={[{ name: "count", color: "grape.6" }]} />
                )}
              </Panel>
              <Panel title="Top countries">
                {a.signalsByCountry.length === 0 ? <Text c="dimmed" size="sm">No data.</Text> : (
                  <BarChart h={240} data={a.signalsByCountry} dataKey="key" orientation="vertical" series={[{ name: "count", color: "orange.6" }]} />
                )}
              </Panel>
            </SimpleGrid>

            <Panel title="Top sources by articles">
              <DataTable
                rows={a.topSources}
                getKey={(s) => s.id}
                emptyMessage="No sources."
                columns={[
                  { key: "name", header: "Source", render: (s) => s.name },
                  { key: "articleCount", header: "Articles", render: (s) => s.articleCount },
                ]}
              />
            </Panel>
          </Stack>
        )}
      </AsyncBoundary>
    </>
  );
}
