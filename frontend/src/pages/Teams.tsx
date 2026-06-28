import { Button, Group, Modal, Paper, Select, Stack, TextInput, Text } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import { api, type Team, type User } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, LoadingState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { DataTable } from "../components/DataTable";
import { ConfirmButton } from "../components/ConfirmButton";
import { fmtDate } from "../lib/format";

export function Teams() {
  const state = useAsync<Team[]>(() => api.teams(), []);
  const [createOpen, createCtl] = useDisclosure(false);
  const [manageId, setManageId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const form = useForm({ initialValues: { name: "" }, validate: { name: (v) => (v.trim() ? null : "Name required") } });

  async function create(v: { name: string }) {
    setBusy(true);
    try {
      await api.createTeam(v.name);
      notifications.show({ message: "Team created", color: "green" });
      createCtl.close(); form.reset(); state.reload();
    } catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
    finally { setBusy(false); }
  }

  return (
    <>
      <PageHeader title="Teams" subtitle="Group users for collaboration"
        actions={<Button leftSection={<IconPlus size={16} />} onClick={createCtl.open}>Add team</Button>} />
      <Paper withBorder p="md" radius="md">
        <AsyncBoundary state={state} empty={(r) => r.length === 0}>
          {(rows) => (
            <DataTable rows={rows} getKey={(r) => r.id}
              columns={[
                { key: "name", header: "Name", render: (r) => r.name },
                { key: "memberCount", header: "Members", render: (r) => r.memberCount },
                { key: "createdAt", header: "Created", render: (r) => fmtDate(r.createdAt) },
                { key: "actions", header: "", render: (r: Team) => (
                  <Group gap="xs" wrap="nowrap">
                    <Button size="xs" variant="light" onClick={() => setManageId(r.id)}>Manage</Button>
                    <ConfirmButton label="Delete" message={`Delete team "${r.name}"?`} confirmLabel="Delete"
                      onConfirm={() => api.deleteTeam(r.id)} onDone={state.reload} />
                  </Group>
                ) },
              ]} />
          )}
        </AsyncBoundary>
      </Paper>

      <Modal opened={createOpen} onClose={createCtl.close} title="Add team" centered>
        <form onSubmit={form.onSubmit(create)}>
          <Stack>
            <TextInput label="Name" required {...form.getInputProps("name")} data-testid="team-name" />
            <Group justify="flex-end">
              <Button variant="default" onClick={createCtl.close}>Cancel</Button>
              <Button type="submit" loading={busy}>Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {manageId && <ManageTeam id={manageId} onClose={() => { setManageId(null); state.reload(); }} />}
    </>
  );
}

function ManageTeam({ id, onClose }: { id: string; onClose: () => void }) {
  const team = useAsync<Team | null>(() => api.team(id), [id]);
  const users = useAsync<User[]>(() => api.users(), []);
  const [userId, setUserId] = useState<string | null>(null);
  const [role, setRole] = useState<string>("MEMBER");

  async function add() {
    if (!userId) return;
    try { await api.addTeamMember(id, userId, role); setUserId(null); team.reload(); }
    catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }
  async function remove(uid: string) {
    try { await api.removeTeamMember(id, uid); team.reload(); }
    catch (e) { notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" }); }
  }

  return (
    <Modal opened onClose={onClose} title={team.data?.name ? `Manage ${team.data.name}` : "Manage team"} centered size="lg">
      {team.loading ? <LoadingState /> : (
        <Stack>
          <Group align="flex-end">
            <Select label="Add member" placeholder="Select user" flex={1} searchable
              data={(users.data ?? []).map((u) => ({ value: u.id, label: u.email }))}
              value={userId} onChange={setUserId} data-testid="team-add-user" />
            <Select label="Role" data={["MEMBER", "OWNER"]} value={role} onChange={(v) => setRole(v ?? "MEMBER")} w={120} />
            <Button onClick={add} disabled={!userId}>Add</Button>
          </Group>
          {(team.data?.members ?? []).length === 0 ? (
            <Text c="dimmed" size="sm">No members yet.</Text>
          ) : (
            <DataTable rows={team.data!.members!} getKey={(m) => m.userId}
              columns={[
                { key: "email", header: "Email", render: (m) => m.email },
                { key: "role", header: "Role", render: (m) => m.role },
                { key: "actions", header: "", render: (m) => (
                  <Button size="xs" variant="light" color="red" onClick={() => remove(m.userId)}>Remove</Button>
                ) },
              ]} />
          )}
        </Stack>
      )}
    </Modal>
  );
}
