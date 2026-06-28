import type PgBoss from "pg-boss";
import { getBoss } from "./boss.js";
import {
  QUEUES,
  type FetchSourceJob,
  type ProcessArticleJob,
  type EnrichSignalJob,
  type MatchSignalJob,
  type SendDeliveryJob,
} from "./queues.js";
import { fetchSource } from "../pipeline/fetchSource.js";
import { normalizeRawItem } from "../pipeline/normalize.js";
import { clusterArticle } from "../pipeline/cluster.js";
import { enrichSignal } from "../pipeline/enrichSignal.js";
import { matchSubscriptions, sendDelivery } from "../pipeline/deliver.js";
import { logger } from "../lib/logger.js";

const log = logger("workers");

const DELIVERY_RETRY_LIMIT = 5;

// Enqueue helpers ------------------------------------------------------------

export async function enqueueFetchSource(sourceId: string) {
  const boss = await getBoss();
  await boss.send(QUEUES.fetchSource, { sourceId } satisfies FetchSourceJob, {
    singletonKey: `fetch:${sourceId}`,
  });
}
export async function enqueueProcessArticle(rawItemId: string) {
  const boss = await getBoss();
  await boss.send(QUEUES.processArticle, { rawItemId } satisfies ProcessArticleJob);
}
export async function enqueueEnrichSignal(signalId: string) {
  const boss = await getBoss();
  await boss.send(QUEUES.enrichSignal, { signalId } satisfies EnrichSignalJob);
}
export async function enqueueMatchSignal(signalId: string) {
  const boss = await getBoss();
  await boss.send(QUEUES.matchSignal, { signalId } satisfies MatchSignalJob);
}
export async function enqueueSendDelivery(deliveryId: string) {
  const boss = await getBoss();
  await boss.send(QUEUES.sendDelivery, { deliveryId } satisfies SendDeliveryJob, {
    retryLimit: DELIVERY_RETRY_LIMIT,
    retryBackoff: true,
    retryDelay: 5,
  });
}

// pg-boss v10 delivers an array of jobs to the handler.
function each<T>(handler: (data: T, job: PgBoss.Job<T>) => Promise<void>) {
  return async (jobs: PgBoss.Job<T>[]) => {
    for (const job of jobs) {
      await handler(job.data, job);
    }
  };
}

export async function registerWorkers(): Promise<void> {
  const boss = await getBoss();

  await boss.work<FetchSourceJob>(
    QUEUES.fetchSource,
    each(async ({ sourceId }) => {
      const rawIds = await fetchSource(sourceId);
      for (const id of rawIds) await enqueueProcessArticle(id);
    }),
  );

  await boss.work<ProcessArticleJob>(
    QUEUES.processArticle,
    each(async ({ rawItemId }) => {
      const articleId = await normalizeRawItem(rawItemId);
      if (!articleId) return; // exact duplicate
      const cluster = await clusterArticle(articleId);
      if (!cluster) return;
      await enqueueEnrichSignal(cluster.signalId);
    }),
  );

  await boss.work<EnrichSignalJob>(
    QUEUES.enrichSignal,
    each(async ({ signalId }) => {
      await enrichSignal(signalId);
      await enqueueMatchSignal(signalId);
    }),
  );

  await boss.work<MatchSignalJob>(
    QUEUES.matchSignal,
    each(async ({ signalId }) => {
      const deliveryIds = await matchSubscriptions(signalId);
      for (const id of deliveryIds) await enqueueSendDelivery(id);
    }),
  );

  await boss.work<SendDeliveryJob>(
    QUEUES.sendDelivery,
    each(async ({ deliveryId }, job) => {
      const attempt = (job as { retryCount?: number }).retryCount ?? 0;
      await sendDelivery(deliveryId, attempt >= DELIVERY_RETRY_LIMIT);
    }),
  );

  log.info("workers registered");
}
