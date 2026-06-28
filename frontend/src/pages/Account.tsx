import { Button, Card, Grid, Group, PasswordInput, Stack, Text } from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { useState } from "react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { PageHeader } from "../components/PageHeader";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

export function Account() {
  const { user } = useAuth();
  const [busy, setBusy] = useState(false);
  const form = useForm({
    initialValues: { oldPassword: "", newPassword: "", confirm: "" },
    validate: {
      newPassword: (v) => (v.length >= 8 ? null : "Min 8 characters"),
      confirm: (v, vals) => (v === vals.newPassword ? null : "Passwords do not match"),
    },
  });

  async function submit(v: typeof form.values) {
    setBusy(true);
    try {
      await api.changePassword(v.oldPassword, v.newPassword);
      notifications.show({ message: "Password changed", color: "green" });
      form.reset();
    } catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
    finally { setBusy(false); }
  }

  return (
    <>
      <PageHeader title="Account" subtitle="Your profile and security" />
      <Grid>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder radius="md">
            <Text fw={700} mb="sm">Profile</Text>
            <Stack gap={4}>
              <Text size="sm"><b>Email:</b> {user?.email}</Text>
              <Text size="sm"><b>Name:</b> {user?.name || "—"}</Text>
              <Group gap="xs"><Text size="sm" fw={700}>Role:</Text>{user && <StatusBadge status={user.role} />}</Group>
              <Text size="sm"><b>Member since:</b> {fmtDate(user?.createdAt)}</Text>
            </Stack>
          </Card>
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder radius="md">
            <Text fw={700} mb="sm">Change password</Text>
            <form onSubmit={form.onSubmit(submit)}>
              <Stack>
                <PasswordInput label="Current password" required {...form.getInputProps("oldPassword")} data-testid="old-password" />
                <PasswordInput label="New password" required {...form.getInputProps("newPassword")} data-testid="new-password" />
                <PasswordInput label="Confirm new password" required {...form.getInputProps("confirm")} data-testid="confirm-password" />
                <Group justify="flex-end"><Button type="submit" loading={busy}>Update password</Button></Group>
              </Stack>
            </form>
          </Card>
        </Grid.Col>
      </Grid>
    </>
  );
}
