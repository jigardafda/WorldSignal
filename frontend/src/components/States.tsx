import { Alert, Button, Center, Loader, Stack, Text } from "@mantine/core";
import { IconAlertTriangle, IconInbox } from "@tabler/icons-react";
import type { ReactNode } from "react";

export function LoadingState({ label = "Loading…" }: { label?: string }) {
  return (
    <Center py="xl" data-testid="loading">
      <Stack align="center" gap="xs">
        <Loader />
        <Text c="dimmed" size="sm">{label}</Text>
      </Stack>
    </Center>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <Alert color="red" icon={<IconAlertTriangle size={18} />} title="Something went wrong" data-testid="error">
      <Stack gap="xs" align="flex-start">
        <Text size="sm">{message}</Text>
        {onRetry && (
          <Button size="xs" variant="light" color="red" onClick={onRetry}>
            Retry
          </Button>
        )}
      </Stack>
    </Alert>
  );
}

export function EmptyState({ message = "Nothing here yet.", action }: { message?: string; action?: ReactNode }) {
  return (
    <Center py="xl" data-testid="empty">
      <Stack align="center" gap="xs">
        <IconInbox size={32} opacity={0.4} />
        <Text c="dimmed" size="sm">{message}</Text>
        {action}
      </Stack>
    </Center>
  );
}

/** Renders the right state for an async resource, or the children when loaded. */
export function AsyncBoundary<T>({
  state,
  children,
  empty,
}: {
  state: { data: T | null; loading: boolean; error: string | null; reload: () => void };
  children: (data: T) => ReactNode;
  empty?: (data: T) => boolean;
}) {
  if (state.loading && state.data === null) return <LoadingState />;
  if (state.error) return <ErrorState message={state.error} onRetry={state.reload} />;
  if (state.data === null) return <EmptyState />;
  if (empty && empty(state.data)) return <EmptyState />;
  return <>{children(state.data)}</>;
}
