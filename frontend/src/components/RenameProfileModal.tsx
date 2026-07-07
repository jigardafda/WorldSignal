import { Button, Group, Modal, Stack, TextInput } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState } from "react";
import { api, type Subscription } from "../lib/api";

/** Rename a profile (subscription). Reused by the Profiles list and the For You feed. */
export function RenameProfileModal({
  profile,
  onClose,
  onRenamed,
}: {
  profile: Subscription | null;
  onClose: () => void;
  onRenamed: () => void;
}) {
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (profile) setName(profile.name);
  }, [profile]);

  async function save() {
    const trimmed = name.trim();
    if (!trimmed || !profile) return;
    setSaving(true);
    try {
      await api.updateSubscription(profile.id, { name: trimmed });
      notifications.show({ color: "teal", message: "Profile renamed." });
      onRenamed();
      onClose();
    } catch (e) {
      notifications.show({ color: "red", message: e instanceof Error ? e.message : "Couldn't rename the profile" });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal opened={profile !== null} onClose={onClose} title="Rename profile" size="sm">
      <Stack gap="md">
        <TextInput
          data-autofocus
          label="Profile name"
          value={name}
          onChange={(e) => setName(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && save()}
        />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} loading={saving} disabled={!name.trim()}>
            Save
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
