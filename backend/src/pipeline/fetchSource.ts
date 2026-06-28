import { prisma } from "../db/prisma.js";
import { fetchRssSource } from "../ingestion/rss.js";
import { logger } from "../lib/logger.js";

const log = logger("fetch");

/**
 * Fetch a source, persist new RawItems (raw evidence is never overwritten).
 * Returns the ids of newly created raw items to push downstream.
 */
export async function fetchSource(sourceId: string): Promise<string[]> {
  const source = await prisma.source.findUnique({ where: { id: sourceId } });
  if (!source || !source.enabled) return [];

  let items;
  try {
    items = await fetchRssSource(source.url);
  } catch (err) {
    await prisma.source.update({
      where: { id: sourceId },
      data: {
        lastFetchedAt: new Date(),
        lastFailureAt: new Date(),
        failureCount: { increment: 1 },
      },
    });
    log.warn(`fetch failed for ${source.name}: ${(err as Error).message}`);
    return [];
  }

  const newRawItemIds: string[] = [];
  for (const item of items) {
    try {
      // Dedupe at ingestion on (sourceId, sourceGuid). Skip if seen before.
      if (item.sourceGuid) {
        const exists = await prisma.rawItem.findUnique({
          where: { sourceId_sourceGuid: { sourceId, sourceGuid: item.sourceGuid } },
          select: { id: true },
        });
        if (exists) continue;
      }
      const raw = await prisma.rawItem.create({
        data: {
          sourceId,
          sourceGuid: item.sourceGuid,
          rawUrl: item.url,
          rawTitle: item.title,
          rawContent: item.content,
          rawPayload: item.rawPayload as object,
          publishedAt: item.publishedAt,
          status: "PENDING",
        },
      });
      newRawItemIds.push(raw.id);
    } catch {
      // unique violation race — already ingested, skip.
    }
  }

  await prisma.source.update({
    where: { id: sourceId },
    data: {
      lastFetchedAt: new Date(),
      lastSuccessAt: new Date(),
      failureCount: 0,
    },
  });
  log.info(`${source.name}: ${items.length} items, ${newRawItemIds.length} new`);
  return newRawItemIds;
}
