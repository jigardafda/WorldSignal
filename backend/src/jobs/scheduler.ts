import { prisma } from "../db/prisma.js";
import { enqueueFetchSource } from "./workers.js";
import { logger } from "../lib/logger.js";

const log = logger("scheduler");

const TICK_MS = Number(process.env.SCHEDULER_TICK_MS ?? 30_000);
let timer: NodeJS.Timeout | null = null;

/** Enqueue fetches for any enabled source whose crawl interval has elapsed. */
async function tick(): Promise<void> {
  const now = Date.now();
  const sources = await prisma.source.findMany({
    where: { enabled: true },
    select: { id: true, name: true, crawlFrequency: true, lastFetchedAt: true },
    orderBy: { priority: "asc" },
  });

  let due = 0;
  for (const s of sources) {
    const last = s.lastFetchedAt?.getTime() ?? 0;
    if (now - last >= s.crawlFrequency * 1000) {
      await enqueueFetchSource(s.id);
      due++;
    }
  }
  if (due > 0) log.info(`scheduled ${due}/${sources.length} sources`);
}

export function startScheduler(): void {
  if (timer) return;
  // Kick once on boot, then on an interval. singletonKey in enqueue prevents
  // piling up duplicate fetch jobs for the same source.
  void tick().catch((e) => log.error("tick failed", (e as Error).message));
  timer = setInterval(() => {
    void tick().catch((e) => log.error("tick failed", (e as Error).message));
  }, TICK_MS);
  log.info(`scheduler started (tick ${TICK_MS}ms)`);
}

export function stopScheduler(): void {
  if (timer) {
    clearInterval(timer);
    timer = null;
  }
}
