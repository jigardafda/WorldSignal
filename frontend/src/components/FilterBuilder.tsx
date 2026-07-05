import { Group, JsonInput, MultiSelect, NumberInput, Select, Stack, Switch, TagsInput, Text, TextInput } from "@mantine/core";
import { useState } from "react";
import { CATEGORIES, categoryLabel } from "../lib/categories";
import { useCountries } from "../lib/countries";
import { INFLUENCES, SENTIMENTS, SEVERITIES, type SubFilter } from "../lib/subFilter";

interface Props {
  value: SubFilter;
  onChange: (f: SubFilter) => void;
}

/** A visual editor for a subscription filter — dropdowns and inputs instead of
 * raw JSON. Merges are kept raw (the parent cleans on save) so typing isn't
 * disrupted. A toggle exposes the underlying JSON for power users. */
export function FilterBuilder({ value, onChange }: Props) {
  const { list } = useCountries();
  const [jsonMode, setJsonMode] = useState(false);
  const set = (patch: Partial<SubFilter>) => onChange({ ...value, ...patch });

  const toggle = (
    <Switch
      size="sm"
      label="JSON"
      checked={jsonMode}
      onChange={(e) => setJsonMode(e.currentTarget.checked)}
      data-testid="filter-json-toggle"
    />
  );

  if (jsonMode) {
    return (
      <Stack gap="sm" data-testid="filter-builder">
        <Group justify="space-between">
          <Text fw={600} size="sm">Filter (JSON)</Text>
          {toggle}
        </Group>
        <JsonInput
          autosize
          minRows={10}
          value={JSON.stringify(value, null, 2)}
          onChange={(s) => {
            try {
              onChange(JSON.parse(s || "{}"));
            } catch {
              /* keep the last valid value until the JSON parses */
            }
          }}
          data-testid="filter-json"
        />
      </Stack>
    );
  }

  const categoryData = CATEGORIES.map((c) => ({ value: c.code, label: categoryLabel(c.code) }));
  const countryData = list.map((c) => ({ value: c.code, label: `${c.flag} ${c.name}` }));
  const minSelect = (label: string, val: string | undefined, opts: string[], key: "minSeverity" | "minInfluence") => (
    <Select
      label={label}
      data={["Any", ...opts]}
      value={val ?? "Any"}
      onChange={(v) => set({ [key]: !v || v === "Any" ? undefined : v } as Partial<SubFilter>)}
      data-testid={`filter-${key}`}
    />
  );
  const num = (label: string, val: number | undefined, key: "minConfidence" | "minRelevance") => (
    <NumberInput
      label={label}
      min={0}
      max={1}
      step={0.05}
      decimalScale={2}
      placeholder="0.00"
      value={val ?? ""}
      onChange={(v) => set({ [key]: typeof v === "number" ? v : undefined } as Partial<SubFilter>)}
    />
  );

  return (
    <Stack gap="sm" data-testid="filter-builder">
      <Group justify="space-between">
        <Text fw={600} size="sm">Match signals where…</Text>
        {toggle}
      </Group>
      <MultiSelect
        label="Categories" placeholder="Any category" data={categoryData} clearable searchable
        value={value.tags ?? []} onChange={(v) => set({ tags: v })} data-testid="filter-categories"
      />
      <MultiSelect
        label="Countries" placeholder="Any country" data={countryData} clearable searchable
        value={value.countries ?? []} onChange={(v) => set({ countries: v })}
      />
      <MultiSelect
        label="Sentiment" placeholder="Any sentiment" data={SENTIMENTS} clearable
        value={value.sentiment ?? []} onChange={(v) => set({ sentiment: v })}
      />
      <Group grow>
        {minSelect("Min severity", value.minSeverity, SEVERITIES, "minSeverity")}
        {minSelect("Min influence", value.minInfluence, INFLUENCES, "minInfluence")}
      </Group>
      <Group grow>
        {num("Min confidence", value.minConfidence, "minConfidence")}
        {num("Min relevance", value.minRelevance, "minRelevance")}
      </Group>
      <TagsInput
        label="Regions / states" placeholder="Type a region, press Enter"
        value={value.regions ?? []} onChange={(v) => set({ regions: v })}
      />
      <TagsInput
        label="Entities" placeholder="e.g. NATO, FEMA, a person…"
        value={value.entities ?? []} onChange={(v) => set({ entities: v })}
      />
      <TextInput
        label="Keyword" placeholder="substring of the title or summary"
        value={value.keyword ?? ""} onChange={(e) => set({ keyword: e.currentTarget.value })}
        data-testid="filter-keyword"
      />
    </Stack>
  );
}
