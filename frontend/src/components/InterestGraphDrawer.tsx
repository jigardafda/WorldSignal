import { ActionIcon, Button, Drawer, Group, Select, Slider, Stack, Text, TextInput, ThemeIcon } from "@mantine/core";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import { notifications } from "@mantine/notifications";
import { useState } from "react";
import { api } from "../lib/api";
import { humanizeReason, reasonColor } from "../lib/relevanceUi";

const DIMENSIONS = [
  { value: "entity", label: "Entity — a brand, competitor, or person" },
  { value: "tag", label: "Topic — a taxonomy code, e.g. DISASTER" },
  { value: "country", label: "Place — an ISO country, e.g. US" },
  { value: "keyword", label: "Keyword — a term to track" },
  { value: "sentiment", label: "Sentiment — POSITIVE / NEGATIVE" },
];

/** Edit a profile's weighted interest graph — the control that drives ranking.
 * Weights (1–5) set how much each interest pushes a signal up the feed. */
export function InterestGraphDrawer({
  opened,
  onClose,
  subscriptionId,
  profileName,
  initial,
  onSaved,
}: {
  opened: boolean;
  onClose: () => void;
  subscriptionId: string;
  profileName: string;
  initial: Record<string, number>;
  onSaved: () => void;
}) {
  const [interests, setInterests] = useState<Record<string, number>>(initial);
  const [dim, setDim] = useState<string>("entity");
  const [val, setVal] = useState("");
  const [saving, setSaving] = useState(false);

  // Re-seed when opening a different profile.
  const [seed, setSeed] = useState(subscriptionId);
  if (seed !== subscriptionId) {
    setSeed(subscriptionId);
    setInterests(initial);
  }

  function add() {
    const v = val.trim();
    if (!v) return;
    const value = dim === "sentiment" || dim === "tag" ? v.toUpperCase() : v;
    setInterests((prev) => ({ ...prev, [`${dim}:${value}`]: prev[`${dim}:${value}`] ?? 3 }));
    setVal("");
  }
  function setWeight(key: string, w: number) {
    setInterests((prev) => ({ ...prev, [key]: w }));
  }
  function remove(key: string) {
    setInterests((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  }
  async function save() {
    setSaving(true);
    try {
      await api.setSubscriptionInterests(subscriptionId, interests);
      notifications.show({ color: "teal", message: "Interests saved — your feed will re-rank." });
      onSaved();
      onClose();
    } catch (e) {
      notifications.show({ color: "red", message: e instanceof Error ? e.message : "Couldn't save interests" });
    } finally {
      setSaving(false);
    }
  }

  const rows = Object.entries(interests).sort((a, b) => b[1] - a[1]);

  return (
    <Drawer opened={opened} onClose={onClose} position="right" size="md" title={`What ${profileName} watches`}>
      <Stack gap="lg">
        <Text size="sm" c="dimmed">
          Add what this profile cares about and weight it. Higher weight ranks higher. Feedback tunes
          these over time.
        </Text>

        <Group align="flex-end" gap="xs" wrap="nowrap">
          <Select data-testid="dim" value={dim} onChange={(v) => setDim(v ?? "entity")} data={DIMENSIONS} style={{ flex: 1 }} label="Dimension" />
          <TextInput
            label="Value"
            placeholder="e.g. Adidas, DISASTER, US"
            value={val}
            onChange={(e) => setVal(e.currentTarget.value)}
            onKeyDown={(e) => e.key === "Enter" && add()}
            style={{ flex: 1 }}
          />
          <Button onClick={add} leftSection={<IconPlus size={16} />}>
            Add
          </Button>
        </Group>

        <Stack gap="sm">
          {rows.length === 0 && (
            <Text size="sm" c="dimmed" ta="center" py="md">
              No interests yet — add a few above to shape the ranking.
            </Text>
          )}
          {rows.map(([key, w]) => {
            const chip = humanizeReason(key);
            return (
              <Group key={key} gap="sm" wrap="nowrap" data-testid="interest-row">
                <ThemeIcon variant="light" color={reasonColor(chip.kind)} size="sm" radius="sm">
                  <span style={{ fontSize: 9 }}>{chip.kind[0].toUpperCase()}</span>
                </ThemeIcon>
                <Text size="sm" style={{ flex: 1, minWidth: 0 }} truncate>
                  {chip.label}
                </Text>
                <Slider
                  value={w}
                  onChange={(v) => setWeight(key, v)}
                  min={1}
                  max={5}
                  step={1}
                  marks={[1, 2, 3, 4, 5].map((n) => ({ value: n }))}
                  style={{ width: 130 }}
                  label={(v) => `×${v}`}
                />
                <ActionIcon variant="subtle" color="red" aria-label="Remove interest" onClick={() => remove(key)}>
                  <IconTrash size={16} />
                </ActionIcon>
              </Group>
            );
          })}
        </Stack>

        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} loading={saving}>
            Save interests
          </Button>
        </Group>
      </Stack>
    </Drawer>
  );
}
