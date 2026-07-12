import { Alert, Box, Button, Center, Flex, Group, PasswordInput, Paper, Stack, Text, TextInput, ThemeIcon, Title } from "@mantine/core";
import { useForm } from "@mantine/form";
import { IconBolt, IconChecks, IconWorldBolt } from "@tabler/icons-react";
import { LogoMark } from "../components/Logo";
import { useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";

const HIGHLIGHTS = [
  { icon: <IconWorldBolt size={18} />, title: "Real-time global signals", body: "The world's public news, distilled into clean events." },
  { icon: <IconChecks size={18} />, title: "Deduplicated & enriched", body: "Clustered, classified and scored for relevance." },
  { icon: <IconBolt size={18} />, title: "Delivered your way", body: "REST API, webhooks, streaming and email digests." },
];

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
    <Flex mih="100vh">
      {/* Brand hero — sets the tone before the customer even signs in. */}
      <Box
        visibleFrom="md"
        style={{
          flex: 1.1,
          position: "relative",
          overflow: "hidden",
          background: "linear-gradient(135deg, #10245f 0%, #2f6df6 52%, #22c3e6 100%)",
        }}
      >
        <Box style={{ position: "absolute", inset: 0, backgroundImage: "radial-gradient(circle at 1px 1px, rgba(255,255,255,0.14) 1px, transparent 0)", backgroundSize: "22px 22px", opacity: 0.5 }} />
        <Stack justify="space-between" h="100%" p={48} style={{ position: "relative" }}>
          <Group gap="sm">
            <LogoMark size={40} />
            <Text fw={800} fz={26} c="white" style={{ fontFamily: "'Space Grotesk Variable', sans-serif", letterSpacing: "-0.02em" }}>WorldSignal</Text>
          </Group>
          <Stack gap="xl" maw={440}>
            <Title order={1} c="white" style={{ fontSize: 40, lineHeight: 1.1 }}>
              The world's news, as real-time signals.
            </Title>
            <Stack gap="lg">
              {HIGHLIGHTS.map((h) => (
                <Group key={h.title} wrap="nowrap" align="flex-start" gap="md">
                  <ThemeIcon size={38} radius="md" variant="white" color="dark">{h.icon}</ThemeIcon>
                  <div>
                    <Text c="white" fw={600}>{h.title}</Text>
                    <Text c="white" opacity={0.75} size="sm">{h.body}</Text>
                  </div>
                </Group>
              ))}
            </Stack>
          </Stack>
          <Text c="white" opacity={0.6} size="xs">© WorldSignal — global signal intelligence</Text>
        </Stack>
      </Box>

      {/* Sign-in panel */}
      <Center style={{ flex: 1 }} p="xl">
        <Paper withBorder shadow="lg" p="xl" radius="lg" w={400}>
          <Stack align="center" gap={6} mb="lg">
            <Box hiddenFrom="md"><LogoMark size={44} /></Box>
            <Title order={3}><span className="ws-wordmark">WorldSignal</span></Title>
            <Text c="dimmed" size="sm">Sign in to the admin console</Text>
          </Stack>
          <form onSubmit={form.onSubmit(submit)}>
            <Stack>
              {error && <Alert color="red" data-testid="login-error">{error}</Alert>}
              <TextInput label="Email" placeholder="you@example.com" size="md" {...form.getInputProps("email")} data-testid="email" />
              <PasswordInput label="Password" size="md" {...form.getInputProps("password")} data-testid="password" />
              <Button type="submit" size="md" loading={busy} fullWidth mt="xs">Sign in</Button>
            </Stack>
          </form>
        </Paper>
      </Center>
    </Flex>
  );
}
