import { createHmac } from "node:crypto";
import { prisma } from "../db/prisma.js";
import { env } from "../config/env.js";
import { logger } from "../lib/logger.js";
import { SEVERITY_RANK } from "./enrichSignal.js";
import type { Severity } from "../llm/enrich.js";

const log = logger("deliver");

interface SubscriptionFilter {
  tags?: string[];
  countries?: string[];
  minConfidence?: number;
  minSeverity?: Severity;
}

function matchesFilter(
  filter: SubscriptionFilter,
  signal: { confidence: number; severity: Severity; country: string | null; tagCodes: string[] },
): boolean {
  if (filter.minConfidence != null && signal.confidence < filter.minConfidence) return false;
  if (filter.minSeverity && SEVERITY_RANK[signal.severity] < SEVERITY_RANK[filter.minSeverity]) {
    return false;
  }
  if (filter.countries?.length && (!signal.country || !filter.countries.includes(signal.country))) {
    return false;
  }
  if (filter.tags?.length) {
    // Match if any signal tag is at or under a requested tag prefix (domain or leaf).
    const hit = signal.tagCodes.some((code) =>
      filter.tags!.some((want) => code === want || code.startsWith(`${want}.`)),
    );
    if (!hit) return false;
  }
  return true;
}

/** Match a published signal against subscriptions; create PENDING delivery rows. */
export async function matchSubscriptions(signalId: string): Promise<string[]> {
  const signal = await prisma.signal.findUnique({
    where: { id: signalId },
    include: { tags: { include: { tag: true } } },
  });
  if (!signal) return [];

  const tagCodes = signal.tags.map((t) => t.tag.code);
  const subs = await prisma.subscription.findMany({ where: { enabled: true } });

  const deliveryIds: string[] = [];
  for (const sub of subs) {
    const filter = sub.filter as SubscriptionFilter;
    const ok = matchesFilter(filter, {
      confidence: signal.confidence,
      severity: signal.severity as Severity,
      country: signal.country,
      tagCodes,
    });
    if (!ok) continue;

    const payload = buildEnvelope(sub.id, signal, tagCodes);
    try {
      const delivery = await prisma.deliveryEvent.create({
        data: {
          subscriptionId: sub.id,
          signalId,
          channel: sub.channel,
          payload,
          status: "PENDING",
        },
      });
      deliveryIds.push(delivery.id);
    } catch {
      // unique (subscriptionId, signalId) — already queued/delivered, skip.
    }
  }
  if (deliveryIds.length) log.info(`signal ${signalId} matched ${deliveryIds.length} subscriptions`);
  return deliveryIds;
}

function buildEnvelope(subscriptionId: string, signal: any, tagCodes: string[]) {
  return {
    schema_version: "2026-06-01",
    event_type: "signal.published",
    event_id: `evt_${signal.id}_${subscriptionId}`,
    created_at: new Date().toISOString(),
    subscription_id: subscriptionId,
    data: {
      signal_id: signal.id,
      title: signal.title,
      summary: signal.summary,
      status: signal.status,
      severity: signal.severity,
      confidence: signal.confidence,
      country: signal.country,
      tags: tagCodes,
      source_count: signal.sourceCount,
      first_seen_at: signal.firstSeenAt,
      last_seen_at: signal.lastSeenAt,
    },
  };
}

export function signPayload(body: string): string {
  return "sha256=" + createHmac("sha256", env.WEBHOOK_SIGNING_SECRET).update(body).digest("hex");
}

/** Send one delivery. Throws on failure so pg-boss retries; marks terminal state. */
export async function sendDelivery(deliveryId: string, isFinalAttempt: boolean): Promise<void> {
  const delivery = await prisma.deliveryEvent.findUnique({
    where: { id: deliveryId },
    include: { subscription: true },
  });
  if (!delivery || delivery.status === "SENT") return;

  await prisma.deliveryEvent.update({
    where: { id: deliveryId },
    data: { attempts: { increment: 1 } },
  });

  // POLLING subscribers just read the API; the PENDING row is the inbox.
  if (delivery.channel === "POLLING") {
    await prisma.deliveryEvent.update({
      where: { id: deliveryId },
      data: { status: "SENT", deliveredAt: new Date() },
    });
    return;
  }

  const config = delivery.subscription.config as { url?: string };
  if (!config.url) {
    await prisma.deliveryEvent.update({
      where: { id: deliveryId },
      data: { status: "FAILED", failedAt: new Date(), errorMessage: "no webhook url configured" },
    });
    return;
  }

  const body = JSON.stringify(delivery.payload);
  const envelope = delivery.payload as { event_id: string };
  try {
    const res = await fetch(config.url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-WorldSignal-Event-Id": envelope.event_id,
        "X-WorldSignal-Signature": signPayload(body),
        "X-WorldSignal-Timestamp": new Date().toISOString(),
        "X-WorldSignal-Attempt": String(delivery.attempts + 1),
      },
      body,
      signal: AbortSignal.timeout(10000),
    });
    if (!res.ok) throw new Error(`webhook responded ${res.status}`);
    await prisma.deliveryEvent.update({
      where: { id: deliveryId },
      data: { status: "SENT", deliveredAt: new Date(), errorMessage: null },
    });
    log.info(`delivered ${deliveryId} -> ${config.url}`);
  } catch (err) {
    const message = (err as Error).message;
    await prisma.deliveryEvent.update({
      where: { id: deliveryId },
      data: {
        status: isFinalAttempt ? "DEAD_LETTERED" : "RETRYING",
        failedAt: new Date(),
        errorMessage: message,
      },
    });
    log.warn(`delivery ${deliveryId} failed (final=${isFinalAttempt}): ${message}`);
    if (!isFinalAttempt) throw err; // let pg-boss retry
  }
}
