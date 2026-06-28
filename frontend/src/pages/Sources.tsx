import { Button, Group, Modal, NumberInput, Paper, Pagination, Select, Stack, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Source, type SourceFilter, type SourceCoverage } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { HealthBadge, ValidationBadge } from "../components/badges";

const PAGE_SIZE = 25;
const REGIONS = ["Europe", "North America", "South America", "Central America", "Caribbean", "Middle East", "Africa", "South Asia", "Southeast Asia", "East Asia", "Central Asia", "Oceania", "Global"];
const SCOPES = ["GLOBAL", "CONTINENTAL", "REGIONAL", "NATIONAL", "STATE", "CITY"];
const VALIDATION = ["VALID", "INVALID", "PENDING"];
const ORG_TYPES = ["GOVERNMENT", "PUBLIC", "PRIVATE", "INDEPENDENT"];
const SOURCE_TYPES = ["RSS", "ATOM", "AGGREGATOR", "GOVERNMENT_FEED", "SECURITY_ADVISORY", "RESEARCH_FEED", "PRESS_RELEASE"];

function opts(bs?: { key: string; count: number }[]): string[] {
  return (bs ?? []).map((b) => b.key).filter((k) => k && k !== "(none)").sort();
}

export function Sources() {
  const navigate = useNavigate();
  const { hasPerm } = useAuth();
  const canWrite = hasPerm("sources:write");

  const [pendingSearch, setPendingSearch] = useState("");
  const [search, setSearch] = useState("");
  const [region, setRegion] = useState<string | null>(null);
  const [scope, setScope] = useState<string | null>(null);
  const [validationStatus, setValidationStatus] = useState<string | null>(null);
  const [sourceType, setSourceType] = useState<string | null>(null);
  const [orgType, setOrgType] = useState<string | null>(null);
  const [language, setLanguage] = useState<string | null>(null);
  const [industry, setIndustry] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const filter: SourceFilter = {};
  if (search) filter.search = search;
  if (region) filter.region = region;
  if (scope) filter.scope = scope;
  if (validationStatus) filter.validationStatus = validationStatus;
  if (sourceType) filter.sourceType = sourceType;
  if (orgType) filter.orgType = orgType;
  if (language) filter.language = language;
  if (industry) filter.industry = industry;
  const deps = [search, region, scope, validationStatus, sourceType, orgType, language, industry];

  const list = useAsync<Source[]>(() => api.sources(filter, PAGE_SIZE, (page - 1) * PAGE_SIZE), [...deps, page]);
  const count = useAsync<number>(() => api.sourceCount(filter), deps);
  const coverage = useAsync<SourceCoverage>(() => api.sourceCoverage(), []);
  const totalPages = Math.max(1, Math.ceil((count.data ?? 0) / PAGE_SIZE));

  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: { name: "", url: "", country: "", type: "RSS", priority: 2, crawlFrequency: 900, credibility: 0.5 },
    validate: {
      name: (v) => (v.trim() ? null : "Name is required"),
      url: (v) => (/^https?:\/\//.test(v) ? null : "Enter a valid http(s) URL"),
    },
  });

  function resetPage() { setPage(1); }

  async function create(values: typeof form.values) {
    setBusy(true);
    try {
      await api.createSource({
        name: values.name, url: values.url, type: values.type,
        country: values.country || null, priority: values.priority,
        crawlFrequency: values.crawlFrequency, credibility: values.credibility,
      });
      notifications.show({ message: "Source created", color: "green" });
      close();
      form.reset();
      list.reload();
      count.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    } finally {
      setBusy(false);
    }
  }

  async function act(fn: () => Promise<unknown>, msg: string) {
    try {
      await fn();
      notifications.show({ message: msg, color: "green" });
      list.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    }
  }

  return (
    <>
      <PageHeader
        title="Sources"
        subtitle={count.data != null ? `${count.data.toLocaleString()} sources match` : "Validated global news & knowledge feeds"}
        actions={
          <Group>
            <Button variant="default" onClick={() => navigate("/coverage")}>Coverage</Button>
            {canWrite && <Button leftSection={<IconPlus size={16} />} onClick={open}>Add source</Button>}
          </Group>
        }
      />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <Group align="flex-end" gap="sm">
            <TextInput
              placeholder="Search name, publisher or URL…"
              value={pendingSearch}
              onChange={(e) => setPendingSearch(e.currentTarget.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { setSearch(pendingSearch); resetPage(); } }}
              data-testid="source-search"
              flex={1}
            />
            <Select placeholder="Region" clearable data={REGIONS} value={region} onChange={(v) => { setRegion(v); resetPage(); }} data-testid="source-region" w={150} />
            <Select placeholder="Scope" clearable data={SCOPES} value={scope} onChange={(v) => { setScope(v); resetPage(); }} w={130} />
            <Select placeholder="Validation" clearable data={VALIDATION} value={validationStatus} onChange={(v) => { setValidationStatus(v); resetPage(); }} w={130} />
            <Select placeholder="Type" clearable data={SOURCE_TYPES} value={sourceType} onChange={(v) => { setSourceType(v); resetPage(); }} w={150} />
            <Select placeholder="Org" clearable data={ORG_TYPES} value={orgType} onChange={(v) => { setOrgType(v); resetPage(); }} w={140} />
            <Select placeholder="Language" clearable searchable data={opts(coverage.data?.byLanguage)} value={language} onChange={(v) => { setLanguage(v); resetPage(); }} w={130} />
            <Select placeholder="Industry" clearable searchable data={opts(coverage.data?.byIndustry)} value={industry} onChange={(v) => { setIndustry(v); resetPage(); }} w={160} />
          </Group>
        </Paper>

        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={list} empty={(rows) => rows.length === 0}>
            {(rows) => (
              <DataTable
                rows={rows}
                getKey={(r) => r.id}
                onRowClick={(r) => navigate(`/sources/${r.id}`)}
                columns={[
                  { key: "name", header: "Name", render: (r) => r.name },
                  { key: "country", header: "Country", render: (r) => r.country ?? "—" },
                  { key: "region", header: "Region", render: (r) => r.region ?? "—" },
                  { key: "languages", header: "Lang", render: (r) => (r.languages ?? []).join(", ") || (r.language ?? "—") },
                  { key: "sourceType", header: "Type", render: (r) => r.sourceType ?? r.type },
                  { key: "industry", header: "Industry", render: (r) => r.industry ?? "—" },
                  { key: "health", header: "Health", render: (r) => <HealthBadge score={r.healthScore} /> },
                  { key: "validation", header: "Validation", render: (r) => <ValidationBadge status={r.validationStatus} /> },
                  { key: "failureCount", header: "Fails", render: (r) => r.failureCount },
                  ...(canWrite ? [{
                    key: "actions", header: "", render: (r: Source) => (
                      <Group gap="xs" onClick={(e) => e.stopPropagation()} wrap="nowrap">
                        <Button size="xs" variant="light" onClick={() => act(() => api.triggerFetch(r.id), `Queued ${r.name}`)}>Fetch</Button>
                        <Button size="xs" variant="light" color="grape" onClick={() => act(() => api.revalidateSource(r.id), "Revalidated")}>Revalidate</Button>
                        <ConfirmButton label="Delete" message={`Delete source "${r.name}"? This removes its raw items and articles.`}
                          confirmLabel="Delete" onConfirm={() => api.deleteSource(r.id)} onDone={() => { list.reload(); count.reload(); }} />
                      </Group>
                    ),
                  }] : []),
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

      <Modal opened={opened} onClose={close} title="Add source" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" required {...form.getInputProps("name")} data-testid="src-name" />
            <TextInput label="RSS/Atom URL" required {...form.getInputProps("url")} data-testid="src-url" />
            <TextInput label="Country" placeholder="US" {...form.getInputProps("country")} />
            <Select label="Type" data={["RSS", "ATOM"]} {...form.getInputProps("type")} />
            <NumberInput label="Priority (0=highest)" min={0} max={5} {...form.getInputProps("priority")} />
            <NumberInput label="Crawl frequency (s)" min={30} {...form.getInputProps("crawlFrequency")} />
            <NumberInput label="Credibility" min={0} max={1} step={0.05} decimalScale={2} {...form.getInputProps("credibility")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>Cancel</Button>
              <Button type="submit" loading={busy}>Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </>
  );
}
