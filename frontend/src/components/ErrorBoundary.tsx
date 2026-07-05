import { Button, Center, Stack, Text, ThemeIcon } from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  /** When this value changes, a caught error is cleared (e.g. pass the route so
   * navigating away recovers automatically). */
  resetKey?: unknown;
}
interface State {
  error: Error | null;
}

/** Catches render/lifecycle errors in its subtree and shows a fallback instead
 * of letting a single component white-screen the whole app. Placed around the
 * routed page content so the header/nav stay usable. */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Surface for debugging; a production deployment would forward to an error
    // reporting service here.
    console.error("Unhandled UI error:", error, info.componentStack);
  }

  componentDidUpdate(prev: Props) {
    if (this.state.error && prev.resetKey !== this.props.resetKey) {
      this.setState({ error: null });
    }
  }

  render() {
    if (this.state.error) {
      return (
        <Center h="100%" p="xl" data-testid="error-boundary">
          <Stack align="center" gap="sm" maw={440}>
            <ThemeIcon size={48} radius="xl" color="red" variant="light">
              <IconAlertTriangle size={28} />
            </ThemeIcon>
            <Text fw={700} size="lg">Something went wrong</Text>
            <Text size="sm" c="dimmed" ta="center">
              This view hit an unexpected error. Reload to try again, or pick another page from the menu.
            </Text>
            <Button variant="light" onClick={() => window.location.reload()} data-testid="error-reload">
              Reload
            </Button>
          </Stack>
        </Center>
      );
    }
    return this.props.children;
  }
}
