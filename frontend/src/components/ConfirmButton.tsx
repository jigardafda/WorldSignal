import { Button, Group, Modal, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useState, type ReactNode } from "react";
import { notifications } from "@mantine/notifications";

/** A button that asks for confirmation, runs an async action, and surfaces errors. */
export function ConfirmButton({
  label,
  title = "Are you sure?",
  message,
  confirmLabel = "Confirm",
  color = "red",
  variant = "light",
  size = "xs",
  onConfirm,
  onDone,
}: {
  label: ReactNode;
  title?: string;
  message: string;
  confirmLabel?: string;
  color?: string;
  variant?: string;
  size?: string;
  onConfirm: () => Promise<unknown>;
  onDone?: () => void;
}) {
  const [opened, { open, close }] = useDisclosure(false);
  const [busy, setBusy] = useState(false);

  async function run() {
    setBusy(true);
    try {
      await onConfirm();
      notifications.show({ message: "Done", color: "green" });
      close();
      onDone?.();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Action failed", color: "red" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Button size={size} variant={variant} color={color} onClick={open}>{label}</Button>
      <Modal opened={opened} onClose={close} title={title} centered>
        <Text size="sm" mb="md">{message}</Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={close} disabled={busy}>Cancel</Button>
          <Button color={color} loading={busy} onClick={run}>{confirmLabel}</Button>
        </Group>
      </Modal>
    </>
  );
}
