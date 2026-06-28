import { Paper, SimpleGrid, Stack, Title } from "@mantine/core";
import { BarChart, DonutChart } from "@mantine/charts";
import { api, type SourceCoverage, type Bucket } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { StatCard } from "../components/StatCard";
import { DataTable } from "../components/DataTable";
import { useCountries } from "../lib/countries";

const DONUT = ["blue", "teal", "grape", "orange", "red", "cyan", "lime", "violet", "indigo", "pink"];

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Paper withBorder p="md" radius="md">
      <Title order={5} mb="sm">{title}</Title>
      {children}
    </Paper>
  );
}

function sum(bs: Bucket[]): number {
  return bs.reduce((n, b) => n + b.count, 0);
}

// barHeight scales a vertical bar chart so every category gets a visible bar.
function barHeight(n: number): number {
  return Math.max(240, n * 26 + 40);
}

// industries returns the named industry buckets (drops the "(none)" bucket).
function industries(c: SourceCoverage): Bucket[] {
  return c.byIndustry.filter((b) => b.key !== "(none)");
}

export function Coverage() {
  const state = useAsync<SourceCoverage>(() => api.sourceCoverage(), []);
  const { byCode } = useCountries();
  return (
    <>
      <PageHeader title="Source Coverage" subtitle="Global breadth of the validated source registry" />
      <AsyncBoundary state={state}>
        {(c) => {
          const valid = c.byValidation.find((b) => b.key === "VALID")?.count ?? 0;
          return (
            <Stack gap="lg">
              <SimpleGrid cols={{ base: 2, sm: 3, lg: 6 }}>
                <StatCard label="Total sources" value={sum(c.byValidation)} />
                <StatCard label="Validated" value={valid} color="green" />
                <StatCard label="Countries" value={c.byCountry.filter((b) => b.key !== "(none)").length} color="blue" />
                <StatCard label="Languages" value={c.byLanguage.length} color="grape" />
                <StatCard label="Regions" value={c.byRegion.filter((b) => b.key !== "(none)").length} color="teal" />
                <StatCard label="Industries" value={c.byIndustry.filter((b) => b.key !== "(none)").length} color="orange" />
              </SimpleGrid>

              <SimpleGrid cols={{ base: 1, md: 2 }}>
                <Panel title="By region">
                  <BarChart h={barHeight(c.byRegion.length)} data={c.byRegion} dataKey="key" orientation="vertical" series={[{ name: "count", color: "blue.6" }]} yAxisProps={{ width: 120, interval: 0 }} />
                </Panel>
                <Panel title="By geographic scope">
                  <DonutChart h={280} withLabels data={c.byScope.map((b, i) => ({ name: b.key, value: b.count, color: DONUT[i % DONUT.length] }))} />
                </Panel>
                <Panel title="By organization type">
                  <DonutChart h={280} withLabels data={c.byOrgType.filter((b) => b.key !== "(none)").map((b, i) => ({ name: b.key, value: b.count, color: DONUT[i % DONUT.length] }))} />
                </Panel>
                <Panel title="By source type">
                  <BarChart h={barHeight(c.bySourceType.filter((b) => b.key !== "(none)").length)} data={c.bySourceType.filter((b) => b.key !== "(none)")} dataKey="key" orientation="vertical" series={[{ name: "count", color: "grape.6" }]} yAxisProps={{ width: 150, interval: 0 }} />
                </Panel>
              </SimpleGrid>

              {/* Full-width with height scaled to the number of bars so every
                  language/industry is visible (no truncation). */}
              <Panel title={`Languages (${c.byLanguage.length})`}>
                <BarChart h={barHeight(c.byLanguage.length)} data={c.byLanguage} dataKey="key" orientation="vertical" series={[{ name: "count", color: "teal.6" }]} yAxisProps={{ width: 80, interval: 0 }} />
              </Panel>
              <Panel title={`Industries (${industries(c).length})`}>
                <BarChart h={barHeight(industries(c).length)} data={industries(c)} dataKey="key" orientation="vertical" series={[{ name: "count", color: "orange.6" }]} yAxisProps={{ width: 170, interval: 0 }} />
              </Panel>

              <Panel title="Countries by source count">
                <DataTable
                  rows={c.byCountry.filter((b) => b.key !== "(none)")}
                  getKey={(b) => b.key}
                  emptyMessage="No countries."
                  columns={[
                    { key: "key", header: "Country", render: (b) => { const c = byCode[b.key]; return c ? `${c.flag} ${c.name}` : b.key; } },
                    { key: "count", header: "Sources", render: (b) => b.count },
                  ]}
                />
              </Panel>
            </Stack>
          );
        }}
      </AsyncBoundary>
    </>
  );
}
