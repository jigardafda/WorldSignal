import { Badge, Group, Paper, Select, Stack, Text, TextInput } from "@mantine/core";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type AttributeDefinition, type Entity } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";

function typeOptions(dict: AttributeDefinition[] | null | undefined) {
  const d = dict?.find((x) => x.key === "entityType");
  return (d?.values ?? []).map((v) => ({ value: v.code, label: v.label }));
}

export function Entities() {
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [pending, setPending] = useState("");
  const [type, setType] = useState<string | null>(null);

  const dict = useAsync<AttributeDefinition[]>(() => api.attributeDictionary(), []);
  const list = useAsync<Entity[]>(
    () => api.entities({ search: search || undefined, type: type || undefined }, 200),
    [search, type],
  );

  return (
    <>
      <PageHeader title="Entities" subtitle="People, organizations and places extracted across Signals" />
      <Stack gap="md">
        <Paper withBorder p="sm" radius="md">
          <Group>
            <TextInput
              placeholder="Search entities by name…"
              value={pending}
              onChange={(e) => setPending(e.currentTarget.value)}
              onKeyDown={(e) => e.key === "Enter" && setSearch(pending)}
              data-testid="entity-search"
              flex={1}
            />
            <Select placeholder="Type" clearable data={typeOptions(dict.data)} value={type}
              onChange={setType} data-testid="entity-type" />
          </Group>
        </Paper>

        <Paper withBorder p="md" radius="md">
          <AsyncBoundary state={list} empty={(rows) => rows.length === 0}>
            {(rows) => (
              <DataTable
                rows={rows}
                getKey={(e) => `${e.type}:${e.name}`}
                onRowClick={(e) => navigate(`/signals?entity=${encodeURIComponent(e.name)}`)}
                columns={[
                  { key: "name", header: "Entity", render: (e) => <Text fw={500} size="sm">{e.name}</Text> },
                  { key: "type", header: "Type", render: (e) => <Badge variant="light" color="grape">{e.type}</Badge> },
                  { key: "signalCount", header: "Signals", render: (e) => e.signalCount },
                ]}
              />
            )}
          </AsyncBoundary>
        </Paper>
      </Stack>
    </>
  );
}
