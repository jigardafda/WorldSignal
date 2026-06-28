import { Button, Card, Code, Group, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconArrowLeft } from "@tabler/icons-react";
import { useNavigate, useParams } from "react-router-dom";
import { api, type DeliveryDetail as DD } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { useAuth } from "../lib/auth";
import { AsyncBoundary, EmptyState } from "../components/States";
import { PageHeader } from "../components/PageHeader";
import { StatusBadge } from "../components/badges";
import { fmtDate } from "../lib/format";

export function DeliveryDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { hasPerm } = useAuth();
  const state = useAsync<DD | null>(() => api.delivery(id), [id]);

  async function retry() {
    try {
      await api.retryDelivery(id);
      notifications.show({ message: "Re-queued", color: "green" });
      state.reload();
    } catch (e) {
      notifications.show({ message: e instanceof Error ? e.message : "Failed", color: "red" });
    }
  }

  return (
    <>
      <PageHeader title="Delivery" actions={<Button variant="default" leftSection={<IconArrowLeft size={16} />} onClick={() => navigate("/deliveries")}>Back</Button>} />
      <AsyncBoundary state={state}>
        {(d) => (d === null ? <EmptyState message="Delivery not found." /> : (
          <Card withBorder radius="md">
            <Group justify="space-between" mb="sm">
              <Text fw={700}>{d.signalTitle}</Text>
              <Group>
                <StatusBadge status={d.status} />
                {hasPerm("deliveries:retry") && <Button size="xs" variant="light" disabled={d.status === "SENT"} onClick={retry}>Retry</Button>}
              </Group>
            </Group>
            <Stack gap={4}>
              <Text size="sm"><b>Subscription:</b> {d.subscriptionName} ({d.channel})</Text>
              <Text size="sm"><b>Attempts:</b> {d.attempts}</Text>
              <Text size="sm"><b>Created:</b> {fmtDate(d.createdAt)}</Text>
              <Text size="sm"><b>Delivered:</b> {fmtDate(d.deliveredAt)}</Text>
              <Text size="sm"><b>Failed:</b> {fmtDate(d.failedAt)}</Text>
              {d.errorMessage && <Text size="sm" c="red"><b>Error:</b> {d.errorMessage}</Text>}
              <Text size="sm" fw={700} mt="sm">Payload</Text>
              <Code block>{JSON.stringify(d.payload, null, 2)}</Code>
            </Stack>
          </Card>
        ))}
      </AsyncBoundary>
    </>
  );
}
