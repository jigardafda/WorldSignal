import { Badge, Button, Divider, Group, Modal, Paper, Stack, Text, Textarea, TextInput, ThemeIcon } from "@mantine/core";
import { IconFileText, IconSparkles, IconWand } from "@tabler/icons-react";
import { notifications } from "@mantine/notifications";
import { useState } from "react";
import { api, type ProfileDraft } from "../lib/api";
import { humanizeReason, reasonColor } from "../lib/relevanceUi";

type Step = "input" | "review";

const ORIGIN_LABEL: Record<string, string> = { doc: "from document", web: "web research", inferred: "inferred" };
const ORIGIN_COLOR: Record<string, string> = { doc: "grape", web: "blue", inferred: "gray" };

/** Create a profile the AI-native way: paste a brand document, and we draft a
 * ranked, weighted profile you review before saving. A blank profile is one click. */
export function CreateProfileModal({
  opened,
  onClose,
  onCreated,
}: {
  opened: boolean;
  onClose: () => void;
  onCreated: (subscriptionId: string) => void;
}) {
  const [step, setStep] = useState<Step>("input");
  const [text, setText] = useState("");
  const [analyzing, setAnalyzing] = useState(false);
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState<ProfileDraft | null>(null);
  const [name, setName] = useState("");

  function reset() {
    setStep("input");
    setText("");
    setDraft(null);
    setName("");
  }
  function close() {
    reset();
    onClose();
  }

  async function analyze() {
    setAnalyzing(true);
    try {
      const d = await api.draftProfileFromDocument(text);
      setDraft(d);
      setName(d.name);
      setStep("review");
    } catch (e) {
      notifications.show({ color: "red", message: e instanceof Error ? e.message : "Couldn't read that document" });
    } finally {
      setAnalyzing(false);
    }
  }

  async function create(interests: Record<string, number>) {
    if (!name.trim()) {
      notifications.show({ color: "red", message: "Give the profile a name." });
      return;
    }
    setCreating(true);
    try {
      const sub = await api.createSubscription({ name: name.trim(), channel: "POLLING" });
      if (Object.keys(interests).length > 0) {
        await api.setSubscriptionInterests(sub.id, interests);
      }
      notifications.show({ color: "teal", message: `Created “${name.trim()}” — ranking your feed now.` });
      onCreated(sub.id);
      close();
    } catch (e) {
      notifications.show({ color: "red", message: e instanceof Error ? e.message : "Couldn't create the profile" });
    } finally {
      setCreating(false);
    }
  }

  const reasonByKey = new Map((draft?.reasons ?? []).map((r) => [r.key, r]));
  const interestRows = draft ? Object.entries(draft.interests).sort((a, b) => b[1] - a[1]) : [];

  return (
    <Modal opened={opened} onClose={close} size="lg" title="New profile" data-testid="create-profile">
      {step === "input" && (
        <Stack gap="md">
          <Group gap="xs">
            <ThemeIcon variant="light" color="grape" radius="md">
              <IconSparkles size={18} />
            </ThemeIcon>
            <Text fw={600}>Build it from a document</Text>
          </Group>
          <Text size="sm" c="dimmed">
            Paste a brand brief, media kit, product page, or contract. We read it, research the brand, and
            draft a ranked profile — you review before it's saved.
          </Text>
          <Textarea
            data-testid="doc-input"
            value={text}
            onChange={(e) => setText(e.currentTarget.value)}
            placeholder="Paste your document here…"
            autosize
            minRows={6}
            maxRows={14}
          />
          <Group justify="space-between">
            <Button
              variant="subtle"
              color="gray"
              leftSection={<IconFileText size={16} />}
              onClick={() => {
                setName("New profile");
                create({});
              }}
              loading={creating}
            >
              Start blank instead
            </Button>
            <Button leftSection={<IconWand size={16} />} onClick={analyze} loading={analyzing} disabled={text.trim().length < 20}>
              Analyze document
            </Button>
          </Group>
        </Stack>
      )}

      {step === "review" && draft && (
        <Stack gap="md">
          <Group gap="xs">
            <ThemeIcon variant="light" color="teal" radius="md">
              <IconSparkles size={18} />
            </ThemeIcon>
            <Text fw={600}>Here's what we built</Text>
            <Badge variant="light" color={draft.source === "llm" ? "grape" : "gray"} size="sm">
              {draft.source === "llm" ? "AI-drafted" : "auto-drafted"}
            </Badge>
          </Group>
          {draft.summary && (
            <Text size="sm" c="dimmed">
              {draft.summary}
            </Text>
          )}
          <TextInput label="Profile name" value={name} onChange={(e) => setName(e.currentTarget.value)} />

          <div>
            <Text size="xs" fw={600} c="dimmed" tt="uppercase" mb={8} style={{ letterSpacing: "0.06em" }}>
              Watching · {interestRows.length} interests
            </Text>
            <Stack gap={6}>
              {interestRows.map(([key, w]) => {
                const chip = humanizeReason(key);
                const reason = reasonByKey.get(key);
                return (
                  <Paper key={key} withBorder radius="sm" p="xs">
                    <Group justify="space-between" wrap="nowrap" gap="sm">
                      <Group gap="xs" wrap="nowrap" style={{ minWidth: 0 }}>
                        <Badge variant="dot" color={reasonColor(chip.kind)} size="sm">
                          {chip.label}
                        </Badge>
                        {reason?.why && (
                          <Text size="xs" c="dimmed" truncate>
                            {reason.why}
                          </Text>
                        )}
                      </Group>
                      <Group gap="xs" wrap="nowrap" style={{ flex: "none" }}>
                        {reason && (
                          <Badge variant="light" color={ORIGIN_COLOR[reason.origin]} size="xs">
                            {ORIGIN_LABEL[reason.origin]}
                          </Badge>
                        )}
                        <Text size="xs" fw={600} c="dimmed">
                          ×{w}
                        </Text>
                      </Group>
                    </Group>
                  </Paper>
                );
              })}
            </Stack>
          </div>

          <Divider />
          <Group justify="space-between">
            <Text size="xs" c="dimmed">
              Suggested gate · min score {draft.minScore.toFixed(1)} · severity ≥ {draft.minSeverity}
            </Text>
            <Group>
              <Button variant="default" onClick={() => setStep("input")}>
                Back
              </Button>
              <Button onClick={() => create(draft.interests)} loading={creating}>
                Create profile
              </Button>
            </Group>
          </Group>
        </Stack>
      )}
    </Modal>
  );
}
