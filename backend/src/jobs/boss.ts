import PgBoss from "pg-boss";
import { env } from "../config/env.js";
import { QUEUES } from "./queues.js";
import { logger } from "../lib/logger.js";

const log = logger("boss");

let boss: PgBoss | null = null;

export async function getBoss(): Promise<PgBoss> {
  if (boss) return boss;
  boss = new PgBoss({ connectionString: env.DATABASE_URL });
  boss.on("error", (err) => log.error("pg-boss error", err.message));
  await boss.start();
  // pg-boss v10 requires queues to exist before send/work.
  for (const name of Object.values(QUEUES)) {
    await boss.createQueue(name);
  }
  log.info("pg-boss started");
  return boss;
}

export async function stopBoss(): Promise<void> {
  if (boss) {
    await boss.stop({ graceful: true });
    boss = null;
  }
}
