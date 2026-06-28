import { prisma } from "../db/prisma.js";
import { canonicalizeUrl } from "../lib/url.js";
import { contentHash, tokenSetString } from "../lib/text.js";
import { logger } from "../lib/logger.js";

const log = logger("normalize");

/**
 * Turn a RawItem into a normalized Article, applying exact dedupe.
 * Returns the new articleId, or null if it was an exact duplicate.
 */
export async function normalizeRawItem(rawItemId: string): Promise<string | null> {
  const raw = await prisma.rawItem.findUnique({ where: { id: rawItemId } });
  if (!raw) return null;
  if (raw.status === "PARSED" || raw.status === "DUPLICATE") {
    const existing = await prisma.article.findUnique({ where: { rawItemId } });
    return existing?.id ?? null;
  }

  const title = (raw.rawTitle ?? "").trim();
  const body = (raw.rawContent ?? "").trim();
  if (!title) {
    await prisma.rawItem.update({ where: { id: rawItemId }, data: { status: "FAILED" } });
    return null;
  }

  const canonicalUrl = canonicalizeUrl(raw.rawUrl);
  const hash = contentHash(title, body);

  // Exact dedupe: same content hash or same canonical URL already normalized.
  const dup = await prisma.article.findFirst({
    where: {
      OR: [
        { contentHash: hash },
        ...(canonicalUrl ? [{ canonicalUrl }] : []),
      ],
    },
    select: { id: true },
  });
  if (dup) {
    await prisma.rawItem.update({ where: { id: rawItemId }, data: { status: "DUPLICATE" } });
    log.debug(`raw ${rawItemId} is exact duplicate of article ${dup.id}`);
    return null;
  }

  const article = await prisma.article.create({
    data: {
      rawItemId: raw.id,
      sourceId: raw.sourceId,
      canonicalUrl,
      title,
      body,
      summary: body.slice(0, 280) || null,
      publishedAt: raw.publishedAt,
      contentHash: hash,
      tokenSet: tokenSetString(`${title} ${body}`),
    },
  });
  await prisma.rawItem.update({ where: { id: rawItemId }, data: { status: "PARSED" } });
  return article.id;
}
