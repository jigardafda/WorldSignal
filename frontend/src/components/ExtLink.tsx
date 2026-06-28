import { Anchor, Text } from "@mantine/core";
import type { ReactNode } from "react";

/** Returns the URL only if it is a safe http(s) link, else null. Prevents
 * javascript:/data: URL injection from ingested feed data. */
export function safeHref(url: string | null | undefined): string | null {
  if (!url) return null;
  try {
    const u = new URL(url, window.location.origin);
    return u.protocol === "http:" || u.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
}

/** An external link that only renders an anchor for safe http(s) URLs; otherwise
 * renders the label as plain text. */
export function ExtLink({ url, children, size }: { url: string | null | undefined; children?: ReactNode; size?: string }) {
  const href = safeHref(url);
  const label = children ?? url ?? "—";
  if (!href) return <Text span size={size}>{label}</Text>;
  return <Anchor href={href} target="_blank" rel="noreferrer noopener" size={size}>{label}</Anchor>;
}
