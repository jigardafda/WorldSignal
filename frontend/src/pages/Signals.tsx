import { Group, Pagination, Paper, Select, Stack, Text, TextInput } from "@mantine/core";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type AttributeDefinition, type Signal } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfidenceBar, SeverityBadge, SignalIntel, StatusBadge } from "../components/badges";
import { CountrySelect } from "../components/CountrySelect";
import { useCountries, countryDisplay } from "../lib/countries";
import { fmtDate } from "../lib/format";

const PAGE_SIZE = 25;
const STATUSES = ["UNVERIFIED", "DEVELOPING", "CONFIRMED", "DISPUTED", "CORRECTED", "RETRACTED", "RESOLVED"];

// Closed vocabularies mirror the backend attribute dictionary.
const SENTIMENTS = ["POSITIVE", "NEUTRAL", "NEGATIVE", "MIXED"];
const GEO_SCOPES = ["GLOBAL", "MULTINATIONAL", "NATIONAL", "REGIONAL", "LOCAL"];

function dictValues(dict: AttributeDefinition[] | null | undefined, key: string): { value: string; label: string }[] {
  const d = dict?.find((x) => x.key === key);
  return (d?.values ?? []).map((v) => ({ value: v.code, label: v.label }));
}

export function Signals() {
  const navigate = useNavigate();
  const { byCode } = useCountries();
  const [search, setSearch] = useState("");
  const [pendingSearch, setPendingSearch] = useState("");
  const [country, setCountry] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [minConf, setMinConf] = useState<string | null>(null);
  const [sentiment, setSentiment] = useState<string | null>(null);
  const [geoScope, setGeoScope] = useState<string | null>(null);
  const [industry, setIndustry] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const dict = useAsync<AttributeDefinition[]>(() => api.attributeDictionary(), []);

  const filter: Record<string, unknown> = {};
  if (search) filter.search = search;
  if (country) filter.country = country;
  if (status) filter.status = status;
  if (minConf) filter.minConfidence = Number(minConf);
  if (sentiment) filter.sentiment = sentiment;
  if (geoScope) filter.geoScope = geoScope;
  if (industry) filter.industry = industry;

  const deps = [search, country, status, minConf, sentiment, geoScope, industry, page];
  const list = useAsync<Signal[]>(() => api.signals(filter, PAGE_SIZE, (page - 1) * PAGE_SIZE), deps);
  const count = useAsync<number>(() => api.signalCount(filter), deps.slice(0, -1));
  const totalPages = Math.max(1, Math.ceil((count.data ?? 0) / PAGE_SIZE));

  function applySearch() {
    setSearch(pendingSearch);
    setPage(1);
  }
  const reset = (set: (v: string | null) => void) => (v: string | null) => { set(v); setPage(1); };

  return (
    <>
      <PageHeader title="Signals" subtitle={count.data != null ? `${count.data} signals` : "Browse canonical events"} />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <Stack gap="xs">
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
                onChange={reset(setStatus)} data-testid="signal-status" />
              <Select placeholder="Min confidence" clearable
                data={[{ value: "0.5", label: "≥ 50%" }, { value: "0.7", label: "≥ 70%" }, { value: "0.85", label: "≥ 85%" }]}
                value={minConf} onChange={reset(setMinConf)} data-testid="signal-minconf" />
            </Group>
            <Group>
              <CountrySelect placeholder="Country" value={country}
                onChange={reset(setCountry)} data-testid="signal-country" />
              <Select placeholder="Sentiment" clearable data={SENTIMENTS} value={sentiment}
                onChange={reset(setSentiment)} data-testid="signal-sentiment" />
              <Select placeholder="Geo scope" clearable data={GEO_SCOPES} value={geoScope}
                onChange={reset(setGeoScope)} data-testid="signal-geoscope" />
              <Select placeholder="Industry" clearable searchable data={dictValues(dict.data, "industry")} value={industry}
                onChange={reset(setIndustry)} data-testid="signal-industry" flex={1} />
            </Group>
          </Stack>
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
                  { key: "title", header: "Title", render: (r) => (
                    <div>
                      <Text fw={500} size="sm">{r.title}</Text>
                      <SignalIntel signal={r} />
                    </div>
                  ) },
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
