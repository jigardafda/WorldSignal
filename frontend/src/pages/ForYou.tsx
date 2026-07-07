import { ActionIcon, Button, Chip, Group, Menu, SegmentedControl, Stack, Text } from "@mantine/core";
import { IconAdjustments, IconDotsVertical, IconList, IconPencil, IconPlus, IconSparkles, IconTrash } from "@tabler/icons-react";
import { notifications } from "@mantine/notifications";
import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api, type FeedbackAction, type Subscription } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { SignalFeedCard } from "../components/SignalFeedCard";
import { InterestGraphDrawer } from "../components/InterestGraphDrawer";
import { CreateProfileModal } from "../components/CreateProfileModal";
import { RenameProfileModal } from "../components/RenameProfileModal";
import { DeleteProfileModal } from "../components/DeleteProfileModal";
import { rankItems, type RankMode } from "../lib/relevanceUi";

// The feed defaults to hiding pure "background" signals (score < 2 — no interest
// matched, just intrinsic quality) so a profile shows what's relevant to it, not
// the whole firehose. "All" reveals everything.
const RELEVANCE_FILTERS = [
  { label: "All", value: "0" },
  { label: "Relevant", value: "2" },
  { label: "Strong", value: "7" },
];

/** For You — a ranked, explained feed per profile. Pick a profile, see what
 * matters most to it right now, and teach the ranking with feedback. */
export function ForYou() {
  const profilesState = useAsync(() => api.subscriptions(), []);
  const profiles = profilesState.data ?? [];
  const [params] = useSearchParams();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const active = selectedId ?? params.get("profile") ?? profiles[0]?.id ?? null;
  const activeProfile = profiles.find((p) => p.id === active);

  const [rankMode, setRankMode] = useState<RankMode>("relevance");
  const [minScore, setMinScore] = useState("2"); // hide background noise by default
  const [feedNonce, setFeedNonce] = useState(0);
  const [votes, setVotes] = useState<Record<string, FeedbackAction>>({});

  const [createOpen, setCreateOpen] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerInterests, setDrawerInterests] = useState<Record<string, number>>({});
  const [renaming, setRenaming] = useState<Subscription | null>(null);
  const [deleting, setDeleting] = useState<Subscription | null>(null);

  const feedState = useAsync(
    () => (active ? api.subscriptionFeed(active, Number(minScore), 40) : Promise.resolve([])),
    [active, minScore, feedNonce],
  );

  function onFeedback(signalId: string, action: FeedbackAction) {
    if (!active) return;
    if (action !== "OPEN") {
      setVotes((v) => ({ ...v, [signalId]: v[signalId] === action ? (undefined as never) : action }));
    }
    api.recordSignalFeedback(active, signalId, action).catch(() => {
      if (action !== "OPEN") notifications.show({ color: "red", message: "Couldn't save that feedback." });
    });
  }

  async function openInterests() {
    if (!active) return;
    try {
      const interests = await api.subscriptionInterests(active);
      setDrawerInterests(interests ?? {});
      setDrawerOpen(true);
    } catch {
      setDrawerInterests({});
      setDrawerOpen(true);
    }
  }

  const items = feedState.data ? rankItems(feedState.data, rankMode) : [];

  return (
    <Stack gap="lg">
      <PageHeader
        title="For You"
        subtitle="A ranked, explained feed for each profile — the signals that matter to it right now."
        actions={
          <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>
            New profile
          </Button>
        }
      />

      <AsyncBoundary state={profilesState}>
        {() =>
          profiles.length === 0 ? (
            <EmptyState
              message="No profiles yet. Create one from a document and we'll draft a ranked feed for you."
              action={
                <Button leftSection={<IconSparkles size={16} />} onClick={() => setCreateOpen(true)}>
                  Create your first profile
                </Button>
              }
            />
          ) : (
            <Stack gap="md">
              <Chip.Group multiple={false} value={active ?? ""} onChange={(v) => setSelectedId(v as string)}>
                <Group gap="xs">
                  {profiles.map((p) => (
                    <Chip key={p.id} value={p.id} variant="outline" data-testid="profile-chip">
                      {p.name}
                    </Chip>
                  ))}
                </Group>
              </Chip.Group>

              <Group justify="space-between" wrap="wrap" gap="sm">
                <Group gap="xs">
                  <Text size="xs" c="dimmed" fw={600}>
                    Rank by
                  </Text>
                  <SegmentedControl
                    size="xs"
                    value={rankMode}
                    onChange={(v) => setRankMode(v as RankMode)}
                    data={[
                      { label: "Relevance", value: "relevance" },
                      { label: "Recency", value: "recency" },
                    ]}
                  />
                  <SegmentedControl size="xs" value={minScore} onChange={setMinScore} data={RELEVANCE_FILTERS} />
                </Group>
                <Group gap="xs">
                  <Button
                    variant="light"
                    leftSection={<IconAdjustments size={16} />}
                    onClick={openInterests}
                    disabled={!active}
                  >
                    Edit interests
                  </Button>
                  <Menu withinPortal position="bottom-end" withArrow>
                    <Menu.Target>
                      <ActionIcon variant="light" color="gray" size="lg" aria-label="Profile actions" disabled={!activeProfile}>
                        <IconDotsVertical size={18} />
                      </ActionIcon>
                    </Menu.Target>
                    <Menu.Dropdown>
                      <Menu.Item leftSection={<IconPencil size={15} />} onClick={() => activeProfile && setRenaming(activeProfile)}>
                        Rename profile
                      </Menu.Item>
                      <Menu.Item component={Link} to="/profiles" leftSection={<IconList size={15} />}>
                        Manage all profiles
                      </Menu.Item>
                      <Menu.Divider />
                      <Menu.Item color="red" leftSection={<IconTrash size={15} />} onClick={() => activeProfile && setDeleting(activeProfile)}>
                        Delete profile
                      </Menu.Item>
                    </Menu.Dropdown>
                  </Menu>
                </Group>
              </Group>

              <AsyncBoundary state={feedState}>
                {(feed) =>
                  feed.length === 0 || items.length === 0 ? (
                    <EmptyState
                      message={
                        activeProfile
                          ? `No recent signals clear the bar for “${activeProfile.name}”. Lower the relevance filter, or tune its interests.`
                          : "No recent signals."
                      }
                      action={
                        <Button variant="light" leftSection={<IconAdjustments size={16} />} onClick={openInterests}>
                          Edit interests
                        </Button>
                      }
                    />
                  ) : (
                    <Stack gap="sm">
                      {items.map((item) => (
                        <SignalFeedCard
                          key={item.id}
                          item={item}
                          vote={votes[item.id] ?? null}
                          onFeedback={(a) => onFeedback(item.id, a)}
                        />
                      ))}
                    </Stack>
                  )
                }
              </AsyncBoundary>
            </Stack>
          )
        }
      </AsyncBoundary>

      <CreateProfileModal
        opened={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(id) => {
          profilesState.reload();
          setSelectedId(id);
          setFeedNonce((n) => n + 1);
        }}
      />
      {active && activeProfile && (
        <InterestGraphDrawer
          opened={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          subscriptionId={active}
          profileName={activeProfile.name}
          initial={drawerInterests}
          onSaved={() => setFeedNonce((n) => n + 1)}
        />
      )}
      <RenameProfileModal profile={renaming} onClose={() => setRenaming(null)} onRenamed={() => profilesState.reload()} />
      <DeleteProfileModal
        profile={deleting}
        onClose={() => setDeleting(null)}
        onDeleted={() => {
          setSelectedId(null);
          setFeedNonce((n) => n + 1);
          profilesState.reload();
        }}
      />
    </Stack>
  );
}
