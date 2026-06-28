import { prisma } from "../db/prisma.js";
import { enrichArticle, type Severity } from "../llm/enrich.js";
import { logger } from "../lib/logger.js";

const log = logger("enrich");

const SEVERITY_RANK: Record<Severity, number> = { LOW: 0, MEDIUM: 1, HIGH: 2, CRITICAL: 3 };

function clamp01(n: number): number {
  return Math.max(0, Math.min(1, n));
}

/**
 * Enrich a Signal from its representative article + source aggregation.
 * Confidence is NOT just the LLM's — it blends source credibility and the
 * number of independent corroborating sources.
 */
export async function enrichSignal(signalId: string): Promise<void> {
  const signal = await prisma.signal.findUnique({
    where: { id: signalId },
    include: {
      articles: {
        include: { article: { include: { source: true } } },
        orderBy: { addedAt: "asc" },
      },
    },
  });
  if (!signal || signal.articles.length === 0) return;

  // Representative = primary, else the article with the longest body.
  const links = signal.articles;
  const primary =
    links.find((l) => l.relationType === "PRIMARY") ??
    [...links].sort((a, b) => (b.article.body?.length ?? 0) - (a.article.body?.length ?? 0))[0];
  const rep = primary.article;

  const enr = await enrichArticle({
    title: rep.title,
    body: rep.body ?? rep.summary ?? rep.title,
    publisher: rep.source.name,
  });

  // Source aggregation.
  const credibilities = links.map((l) => l.article.source.credibility);
  const avgCred = credibilities.reduce((a, b) => a + b, 0) / credibilities.length;
  const distinctSources = new Set(links.map((l) => l.article.sourceId)).size;
  const independence = Math.min(distinctSources / 5, 1);

  const confidence = clamp01(0.4 * enr.confidence + 0.3 * independence + 0.3 * avgCred);
  const status =
    distinctSources >= 3 ? "CONFIRMED" : distinctSources === 2 ? "DEVELOPING" : "UNVERIFIED";
  const eventType = enr.tags[0]?.code ?? null;

  // Resolve taxonomy tag ids by code.
  const codes = enr.tags.map((t) => t.code);
  const tagRows = await prisma.taxonomyTag.findMany({ where: { code: { in: codes } } });
  const codeToId = new Map(tagRows.map((t) => [t.code, t.id]));

  await prisma.$transaction([
    prisma.signal.update({
      where: { id: signalId },
      data: {
        title: enr.title,
        summary: enr.summary,
        whatHappened: enr.whatHappened || null,
        whyItMatters: enr.whyItMatters || null,
        severity: enr.severity,
        confidence,
        status,
        eventType,
        publishedAt: signal.publishedAt ?? new Date(),
        metadata: {
          ...(signal.metadata as object),
          enrichmentSource: enr.source,
          distinctSources,
        },
      },
    }),
    prisma.signalTag.deleteMany({ where: { signalId } }),
    prisma.signalTag.createMany({
      data: enr.tags
        .filter((t) => codeToId.has(t.code))
        .map((t) => ({ signalId, tagId: codeToId.get(t.code)!, confidence: t.confidence })),
      skipDuplicates: true,
    }),
  ]);

  log.info(
    `signal ${signalId} enriched via ${enr.source}: ${status} conf=${confidence.toFixed(2)} sev=${enr.severity} tags=[${codes.join(",")}]`,
  );
}

export { SEVERITY_RANK };
