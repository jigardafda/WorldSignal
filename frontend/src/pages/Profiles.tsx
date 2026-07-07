import { ActionIcon, Badge, Button, Menu, Paper, Table, Text } from "@mantine/core";
import { IconDotsVertical, IconPencil, IconPlus, IconSparkles, IconTrash } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, type Subscription } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { CreateProfileModal } from "../components/CreateProfileModal";
import { RenameProfileModal } from "../components/RenameProfileModal";
import { DeleteProfileModal } from "../components/DeleteProfileModal";
import { fmtDate } from "../lib/format";

/** Manage your For You profiles — create, rename, and delete. Each profile is a
 * saved, ranked feed with its own interests. */
export function Profiles() {
  const state = useAsync(() => api.subscriptions(), []);
  const [counts, setCounts] = useState<Record<string, number>>({});
  const [createOpen, setCreateOpen] = useState(false);
  const [renaming, setRenaming] = useState<Subscription | null>(null);
  const [deleting, setDeleting] = useState<Subscription | null>(null);
  const navigate = useNavigate();

  // Interest counts per profile (parallel; a handful of profiles).
  useEffect(() => {
    if (!state.data) return;
    let active = true;
    Promise.all(
      state.data.map((p) =>
        api
          .subscriptionInterests(p.id)
          .then((i) => [p.id, Object.keys(i ?? {}).length] as const)
          .catch(() => [p.id, 0] as const),
      ),
    ).then((entries) => {
      if (active) setCounts(Object.fromEntries(entries));
    });
    return () => {
      active = false;
    };
  }, [state.data]);

  return (
    <>
      <PageHeader
        title="Profiles"
        subtitle="Your saved For You feeds — each with its own interests, ranking, and delivery."
        actions={
          <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>
            New profile
          </Button>
        }
      />

      <AsyncBoundary state={state}>
        {(rows) =>
          rows.length === 0 ? (
            <EmptyState
              message="No profiles yet. Create one from a document and we'll draft a ranked feed for you."
              action={
                <Button leftSection={<IconSparkles size={16} />} onClick={() => setCreateOpen(true)}>
                  Create your first profile
                </Button>
              }
            />
          ) : (
            <Paper withBorder radius="md" mt="md">
              <Table verticalSpacing="sm" horizontalSpacing="lg" highlightOnHover>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Profile</Table.Th>
                    <Table.Th>Interests</Table.Th>
                    <Table.Th>Created</Table.Th>
                    <Table.Th w={40} />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {rows.map((p) => (
                    <Table.Tr key={p.id} data-testid="profile-row">
                      <Table.Td>
                        <Text
                          component={Link}
                          to={`/for-you?profile=${p.id}`}
                          fw={600}
                          c="var(--mantine-color-anchor)"
                        >
                          {p.name}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="light" color={counts[p.id] ? "blue" : "gray"} radius="sm">
                          {counts[p.id] ?? 0} interest{counts[p.id] === 1 ? "" : "s"}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" c="dimmed">
                          {fmtDate(p.createdAt)}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Menu withinPortal position="bottom-end" withArrow>
                          <Menu.Target>
                            <ActionIcon variant="subtle" color="gray" aria-label={`Actions for ${p.name}`}>
                              <IconDotsVertical size={16} />
                            </ActionIcon>
                          </Menu.Target>
                          <Menu.Dropdown>
                            <Menu.Item leftSection={<IconSparkles size={15} />} onClick={() => navigate(`/for-you?profile=${p.id}`)}>
                              Open feed
                            </Menu.Item>
                            <Menu.Item leftSection={<IconPencil size={15} />} onClick={() => setRenaming(p)}>
                              Rename
                            </Menu.Item>
                            <Menu.Item color="red" leftSection={<IconTrash size={15} />} onClick={() => setDeleting(p)}>
                              Delete
                            </Menu.Item>
                          </Menu.Dropdown>
                        </Menu>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Paper>
          )
        }
      </AsyncBoundary>

      <CreateProfileModal
        opened={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => state.reload()}
      />
      <RenameProfileModal profile={renaming} onClose={() => setRenaming(null)} onRenamed={() => state.reload()} />
      <DeleteProfileModal profile={deleting} onClose={() => setDeleting(null)} onDeleted={() => state.reload()} />
    </>
  );
}
