import { Alert, Button, Center, Paper, PasswordInput, Stack, Text, TextInput, ThemeIcon, Title } from "@mantine/core";
import { useForm } from "@mantine/form";
import { IconWorld } from "@tabler/icons-react";
import { useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";

export function Login() {
  const { user, login, loading } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const form = useForm({
    initialValues: { email: "", password: "" },
    validate: {
      email: (v) => (/^\S+@\S+$/.test(v) ? null : "Enter a valid email"),
      password: (v) => (v.length >= 1 ? null : "Password is required"),
    },
  });

  if (!loading && user) {
    const to = (location.state as { from?: string } | null)?.from ?? "/";
    return <Navigate to={to} replace />;
  }

  async function submit(values: { email: string; password: string }) {
    setError(null);
    setBusy(true);
    try {
      await login(values.email, values.password);
      navigate("/", { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Center mih="100vh" bg="var(--mantine-color-gray-0)">
      <Paper withBorder shadow="md" p="xl" radius="md" w={380}>
        <Stack align="center" gap="xs" mb="md">
          <ThemeIcon variant="gradient" gradient={{ from: "blue", to: "cyan" }} size={48} radius="md"><IconWorld size={28} /></ThemeIcon>
          <Title order={3}>WorldSignal</Title>
          <Text c="dimmed" size="sm">Sign in to the admin console</Text>
        </Stack>
        <form onSubmit={form.onSubmit(submit)}>
          <Stack>
            {error && <Alert color="red" data-testid="login-error">{error}</Alert>}
            <TextInput label="Email" placeholder="you@example.com" {...form.getInputProps("email")} data-testid="email" />
            <PasswordInput label="Password" {...form.getInputProps("password")} data-testid="password" />
            <Button type="submit" loading={busy} fullWidth>Sign in</Button>
          </Stack>
        </form>
      </Paper>
    </Center>
  );
}
