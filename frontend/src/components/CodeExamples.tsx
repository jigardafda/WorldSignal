import { ActionIcon, Code, CopyButton, Group, ScrollArea, Tabs, Text, Tooltip } from "@mantine/core";
import { IconCheck, IconCopy } from "@tabler/icons-react";
import { useState } from "react";
import { codeFor, LANGUAGES, type Channel, type CodeOpts, type Language } from "../lib/codegen";

interface Props {
  channel: Channel;
  opts: CodeOpts;
}

/** Right-pane panel: per-language, copy-paste client code for the chosen channel,
 * regenerating live as the subscription is configured. */
export function CodeExamples({ channel, opts }: Props) {
  const [lang, setLang] = useState<Language>("python");

  if (channel === "EMAIL") {
    return (
      <Text size="sm" c="dimmed" data-testid="code-examples">
        Email subscriptions are delivered straight to your recipients — there's no client
        code to write. Set the recipients and connector on the left; matched signals arrive
        as HTML+text emails (instant, or batched as an hourly/daily digest).
      </Text>
    );
  }

  const code = codeFor(lang, channel, opts) ?? "";
  const hint =
    channel === "WEBHOOK"
      ? "Run this HTTP server and point the webhook URL at it; it verifies the signature."
      : "Set WORLDSIGNAL_API_KEY (a key with signals:read) and run.";

  return (
    <div data-testid="code-examples">
      <Tabs value={lang} onChange={(v) => setLang((v as Language) ?? "python")}>
        <Tabs.List>
          {LANGUAGES.map((l) => (
            <Tabs.Tab key={l.id} value={l.id} data-testid={`code-tab-${l.id}`}>
              {l.label}
            </Tabs.Tab>
          ))}
        </Tabs.List>
      </Tabs>
      <Group justify="space-between" mt="xs" mb={4} wrap="nowrap">
        <Text size="xs" c="dimmed">{hint}</Text>
        <CopyButton value={code}>
          {({ copied, copy }) => (
            <Tooltip label={copied ? "Copied" : "Copy"} withArrow>
              <ActionIcon variant="subtle" color={copied ? "teal" : "gray"} onClick={copy} aria-label="Copy code" data-testid="code-copy">
                {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
              </ActionIcon>
            </Tooltip>
          )}
        </CopyButton>
      </Group>
      <ScrollArea.Autosize mah="calc(90vh - 230px)" type="auto">
        <Code block style={{ fontSize: "12.5px", lineHeight: 1.55 }} data-testid="code-block">
          {code}
        </Code>
      </ScrollArea.Autosize>
    </div>
  );
}
