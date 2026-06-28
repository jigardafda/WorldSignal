import { prisma } from "../db/prisma.js";
import { jaccard } from "../lib/text.js";
import { logger } from "../lib/logger.js";

const log = logger("cluster");

const SIMILARITY_THRESHOLD = 0.5;
const WINDOW_HOURS = 72;

export interface ClusterResult {
  signalId: string;
  isNew: boolean;
}

/**
 * Attach an Article to an existing Signal (same event) or create a new one.
 * Lightweight token-set Jaccard similarity over a recent window — no vector DB,
 * Postgres only.
 */
export async function clusterArticle(articleId: string): Promise<ClusterResult | null> {
  const article = await prisma.article.findUnique({ where: { id: articleId } });
  if (!article) return null;

  // Already linked? (idempotency on retried jobs)
  const existingLink = await prisma.signalArticle.findFirst({ where: { articleId } });
  if (existingLink) return { signalId: existingLink.signalId, isNew: false };

  const since = new Date(Date.now() - WINDOW_HOURS * 3600 * 1000);
  const candidates = await prisma.signal.findMany({
    where: { lastSeenAt: { gte: since } },
    orderBy: { lastSeenAt: "desc" },
    take: 300,
    select: { id: true, metadata: true, sourceCount: true },
  });

  const articleTokens = article.tokenSet ?? "";
  let best: { id: string; score: number } | null = null;
  for (const c of candidates) {
    const meta = (c.metadata ?? {}) as { tokenSet?: string };
    const score = jaccard(articleTokens, meta.tokenSet ?? "");
    if (!best || score > best.score) best = { id: c.id, score };
  }

  const now = new Date();
  if (best && best.score >= SIMILARITY_THRESHOLD) {
    await prisma.$transaction([
      prisma.signalArticle.create({
        data: {
          signalId: best.id,
          articleId,
          relationType: "SUPPORTING",
          similarityScore: best.score,
        },
      }),
      prisma.signal.update({
        where: { id: best.id },
        data: {
          sourceCount: { increment: 1 },
          lastSeenAt: now,
        },
      }),
    ]);
    log.debug(`article ${articleId} joined signal ${best.id} (score ${best.score.toFixed(2)})`);
    return { signalId: best.id, isNew: false };
  }

  const seen = article.publishedAt ?? now;
  const signal = await prisma.signal.create({
    data: {
      title: article.title,
      summary: article.summary ?? article.title,
      status: "UNVERIFIED",
      firstSeenAt: seen,
      lastSeenAt: now,
      country: article.country,
      sourceCount: 1,
      metadata: { tokenSet: articleTokens },
      articles: {
        create: { articleId, relationType: "PRIMARY", similarityScore: 1 },
      },
    },
  });
  log.debug(`article ${articleId} created new signal ${signal.id}`);
  return { signalId: signal.id, isNew: true };
}
