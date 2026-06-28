import type { FastifyInstance } from "fastify";
import { prisma } from "../db/prisma.js";
import { TAXONOMY } from "../taxonomy/taxonomy.js";
import { enqueueFetchSource } from "../jobs/workers.js";

const signalInclude = {
  tags: { include: { tag: true } },
  articles: { include: { article: { include: { source: true } } } },
} as const;

function serializeSignal(s: any) {
  return {
    id: s.id,
    title: s.title,
    summary: s.summary,
    whatHappened: s.whatHappened,
    whyItMatters: s.whyItMatters,
    status: s.status,
    severity: s.severity,
    confidence: s.confidence,
    eventType: s.eventType,
    country: s.country,
    sourceCount: s.sourceCount,
    firstSeenAt: s.firstSeenAt,
    lastSeenAt: s.lastSeenAt,
    tags: s.tags.map((t: any) => ({ code: t.tag.code, label: t.tag.label, confidence: t.confidence })),
    sources: s.articles.map((a: any) => ({
      publisher: a.article.source.name,
      url: a.article.canonicalUrl,
      publishedAt: a.article.publishedAt,
      relation: a.relationType,
    })),
  };
}

export async function registerRoutes(app: FastifyInstance): Promise<void> {
  // permissive CORS for the local React dev server
  app.addHook("onRequest", async (req, reply) => {
    reply.header("Access-Control-Allow-Origin", "*");
    reply.header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS");
    reply.header("Access-Control-Allow-Headers", "Content-Type, Authorization");
    if (req.method === "OPTIONS") reply.code(204).send();
  });

  app.get("/health", async () => ({ status: "ok", service: "worldsignal" }));

  app.get("/v1/stats", async () => {
    const [sources, articles, signals, sent, pending] = await Promise.all([
      prisma.source.count(),
      prisma.article.count(),
      prisma.signal.count(),
      prisma.deliveryEvent.count({ where: { status: "SENT" } }),
      prisma.deliveryEvent.count({ where: { status: { in: ["PENDING", "RETRYING"] } } }),
    ]);
    return { sources, articles, signals, deliveriesSent: sent, deliveriesPending: pending };
  });

  app.get("/v1/taxonomy", async () => TAXONOMY);

  // ── Signals ───────────────────────────────────────────────────────────────
  app.get("/v1/signals", async (req) => {
    const q = req.query as Record<string, string>;
    const where: any = {};
    if (q.country) where.country = q.country;
    if (q.status) where.status = q.status;
    if (q.minConfidence) where.confidence = { gte: Number(q.minConfidence) };
    if (q.since) where.lastSeenAt = { gte: new Date(q.since) };
    if (q.search) {
      where.OR = [
        { title: { contains: q.search, mode: "insensitive" } },
        { summary: { contains: q.search, mode: "insensitive" } },
      ];
    }
    if (q.tags) {
      where.tags = { some: { tag: { code: { in: q.tags.split(",") } } } };
    }
    const rows = await prisma.signal.findMany({
      where,
      orderBy: { lastSeenAt: "desc" },
      take: Math.min(Number(q.limit ?? 50), 200),
      skip: Number(q.offset ?? 0),
      include: signalInclude,
    });
    return { data: rows.map(serializeSignal) };
  });

  app.get("/v1/signals/:id", async (req, reply) => {
    const { id } = req.params as { id: string };
    const s = await prisma.signal.findUnique({ where: { id }, include: signalInclude });
    if (!s) return reply.code(404).send({ error: "not found" });
    return serializeSignal(s);
  });

  // ── Sources ────────────────────────────────────────────────────────────────
  app.get("/v1/sources", async () => {
    const rows = await prisma.source.findMany({ orderBy: [{ priority: "asc" }, { name: "asc" }] });
    return { data: rows };
  });

  app.post("/v1/sources", async (req, reply) => {
    const b = req.body as any;
    if (!b?.name || !b?.url) return reply.code(400).send({ error: "name and url required" });
    try {
      const source = await prisma.source.create({
        data: {
          name: b.name,
          url: b.url,
          type: b.type ?? "RSS",
          country: b.country ?? null,
          priority: b.priority ?? 5,
          crawlFrequency: b.crawlFrequency ?? 900,
          credibility: b.credibility ?? 0.5,
        },
      });
      await enqueueFetchSource(source.id); // fetch immediately
      return reply.code(201).send(source);
    } catch (e) {
      return reply.code(409).send({ error: "source url already exists" });
    }
  });

  app.patch("/v1/sources/:id", async (req) => {
    const { id } = req.params as { id: string };
    const b = req.body as { enabled?: boolean; priority?: number; crawlFrequency?: number };
    return prisma.source.update({ where: { id }, data: b });
  });

  app.post("/v1/sources/:id/fetch", async (req) => {
    const { id } = req.params as { id: string };
    await enqueueFetchSource(id);
    return { queued: true };
  });

  // ── Subscriptions ────────────────────────────────────────────────────────────
  app.get("/v1/subscriptions", async () => {
    const rows = await prisma.subscription.findMany({
      include: { subscriber: true, _count: { select: { deliveries: true } } },
      orderBy: { createdAt: "desc" },
    });
    return { data: rows };
  });

  app.post("/v1/subscriptions", async (req, reply) => {
    const b = req.body as any;
    if (!b?.name) return reply.code(400).send({ error: "name required" });
    const subscriber = await prisma.subscriber.upsert({
      where: { id: b.subscriberId ?? "__default__" },
      update: {},
      create: { id: "__default__", name: "Default Subscriber" },
    });
    const sub = await prisma.subscription.create({
      data: {
        subscriberId: subscriber.id,
        name: b.name,
        channel: b.channel ?? "WEBHOOK",
        filter: b.filter ?? {},
        config: b.config ?? {},
      },
    });
    return reply.code(201).send(sub);
  });

  app.get("/v1/deliveries", async (req) => {
    const q = req.query as Record<string, string>;
    const rows = await prisma.deliveryEvent.findMany({
      orderBy: { createdAt: "desc" },
      take: Math.min(Number(q.limit ?? 50), 200),
      include: { subscription: true, signal: { select: { title: true } } },
    });
    return { data: rows };
  });
}
