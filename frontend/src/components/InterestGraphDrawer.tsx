import { ActionIcon, Button, Drawer, Group, Select, Slider, Stack, Text, TextInput, ThemeIcon } from "@mantine/core";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import { notifications } from "@mantine/notifications";
import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { CATEGORIES } from "../lib/categories";
import { humanizeReason, reasonColor } from "../lib/relevanceUi";

const DIMENSIONS = [
  { value: "entity", label: "Entity — a brand, competitor, or person" },
  { value: "tag", label: "Topic — a category" },
  { value: "country", label: "Place — an ISO country, e.g. US" },
  { value: "keyword", label: "Keyword — a term to track" },
  { value: "sentiment", label: "Sentiment" },
];
const TOPIC_OPTIONS = CATEGORIES.filter((c) => c.code !== "GENERAL").map((c) => ({ value: c.code, label: c.label }));
const SENTIMENT_OPTIONS = [
  { value: "NEGATIVE", label: "Negative" },
  { value: "POSITIVE", label: "Positive" },
  { value: "NEUTRAL", label: "Neutral" },
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

  // Re-seed from the freshly-loaded interests every time the drawer opens (or the
  // profile changes) so it never shows a previous profile's stale graph.
  useEffect(() => {
    if (opened) {
      setInterests(initial);
      setVal("");
    }
    // `initial` is a fresh object each render; keying on open + profile is correct.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opened, subscriptionId]);

  function add() {
    const v = val.trim();
    if (!v) return;
    const value = dim === "sentiment" || dim === "tag" ? v.toUpperCase() : v;
    const key = `${dim}:${value}`;
    setInterests((prev) => ({ ...prev, [key]: prev[key] ?? 3 }));
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
  const isTag = dim === "tag";
  const isSentiment = dim === "sentiment";

  return (
    <Drawer opened={opened} onClose={onClose} position="right" size="md" title={`What ${profileName} watches`}>
      <Stack gap="lg">
        <Text size="sm" c="dimmed">
          Add what this profile cares about and weight it — higher weight ranks higher. Your 👍/👎 feedback
          tunes these over time.
        </Text>

        <Group align="flex-end" gap="xs" wrap="nowrap">
          <Select
            data-testid="dim"
            value={dim}
            onChange={(v) => {
              setDim(v ?? "entity");
              setVal("");
            }}
            data={DIMENSIONS}
            style={{ flex: 1.2 }}
            label="Add an interest"
            comboboxProps={{ withinPortal: true }}
          />
          {isTag || isSentiment ? (
            <Select
              data-testid="val-select"
              value={val || null}
              onChange={(v) => setVal(v ?? "")}
              data={isTag ? TOPIC_OPTIONS : SENTIMENT_OPTIONS}
              placeholder={isTag ? "Pick a topic" : "Pick sentiment"}
              searchable={isTag}
              style={{ flex: 1 }}
              label="Value"
              comboboxProps={{ withinPortal: true }}
            />
          ) : (
            <TextInput
              data-testid="val"
              label="Value"
              placeholder={dim === "country" ? "e.g. US" : "e.g. Adidas"}
              value={val}
              onChange={(e) => setVal(e.currentTarget.value)}
              onKeyDown={(e) => e.key === "Enter" && add()}
              style={{ flex: 1 }}
            />
          )}
          <Button onClick={add} leftSection={<IconPlus size={16} />} disabled={!val.trim()}>
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
                <Text size="sm" style={{ flex: 1, minWidth: 0 }} truncate title={chip.label}>
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
                  aria-label={`Weight for ${chip.label}`}
                />
                <ActionIcon variant="subtle" color="red" aria-label={`Remove ${chip.label}`} onClick={() => remove(key)}>
                  <IconTrash size={16} />
                </ActionIcon>
              </Group>
            );
          })}
        </Stack>

        <Group justify="space-between">
          <Text size="xs" c="dimmed">
            {rows.length} interest{rows.length === 1 ? "" : "s"}
          </Text>
          <Group>
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button onClick={save} loading={saving}>
              Save interests
            </Button>
          </Group>
        </Group>
      </Stack>
    </Drawer>
  );
}
