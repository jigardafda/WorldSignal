import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useState } from "react";
import { api, type Subscription } from "../lib/api";

/** Confirm and delete a profile. A proper in-app dialog (not window.confirm) so
 * it's styled, accessible, and non-blocking. */
export function DeleteProfileModal({
  profile,
  onClose,
  onDeleted,
}: {
  profile: Subscription | null;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const [busy, setBusy] = useState(false);

  async function del() {
    if (!profile) return;
    setBusy(true);
    try {
      await api.deleteSubscription(profile.id);
      notifications.show({ color: "teal", message: `Deleted “${profile.name}”.` });
      onDeleted();
      onClose();
    } catch (e) {
      notifications.show({ color: "red", message: e instanceof Error ? e.message : "Couldn't delete the profile" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal opened={profile !== null} onClose={onClose} title="Delete profile" size="sm">
      <Stack gap="md">
        <Text size="sm">
          Delete <b>{profile?.name}</b>? This removes the profile and its ranked feed. This can't be undone.
        </Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button color="red" onClick={del} loading={busy}>
            Delete profile
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
