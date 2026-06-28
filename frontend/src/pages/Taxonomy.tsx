import { Badge, Group, Paper, SimpleGrid, Text, Title } from "@mantine/core";
import { api, type Bucket, type TaxonomyNode } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary } from "../components/States";
import { PageHeader } from "../components/PageHeader";

export function Taxonomy() {
  const tree = useAsync<TaxonomyNode[]>(() => api.taxonomy(), []);
  const stats = useAsync<Bucket[]>(() => api.taxonomyStats(), []);
  const counts = new Map((stats.data ?? []).map((b) => [b.key, b.count]));

  return (
    <>
      <PageHeader title="Taxonomy" subtitle="Closed vocabulary the classifier is constrained to (with live signal counts)" />
      <AsyncBoundary state={tree} empty={(n) => n.length === 0}>
        {(nodes) => (
          <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }}>
            {nodes.map((domain) => (
              <Paper withBorder p="md" radius="md" key={domain.code}>
                <Group justify="space-between" mb="sm">
                  <Title order={5}>{domain.label}</Title>
                  <Badge variant="light">{counts.get(domain.code) ?? 0}</Badge>
                </Group>
                <Group gap="xs">
                  {(domain.children ?? []).map((c) => (
                    <Badge key={c.code} variant="outline" color="gray" title={c.label}>
                      {c.code.split(".")[1] ?? c.code} · {counts.get(c.code) ?? 0}
                    </Badge>
                  ))}
                  {(domain.children ?? []).length === 0 && <Text c="dimmed" size="sm">No subcategories</Text>}
                </Group>
              </Paper>
            ))}
          </SimpleGrid>
        )}
      </AsyncBoundary>
    </>
  );
}
