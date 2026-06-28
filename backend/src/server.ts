import Fastify from "fastify";
import { env } from "./config/env.js";
import { hasOpenAI } from "./config/env.js";
import { logger } from "./lib/logger.js";
import { registerRoutes } from "./api/routes.js";
import { yoga } from "./api/graphql.js";
import { getBoss, stopBoss } from "./jobs/boss.js";
import { registerWorkers } from "./jobs/workers.js";
import { startScheduler, stopScheduler } from "./jobs/scheduler.js";
import { prisma } from "./db/prisma.js";

const log = logger("server");

async function buildApi() {
  const app = Fastify({ logger: false });

  // Mount GraphQL Yoga. Yoga parses the body itself, so hand it the raw request.
  app.route({
    url: "/graphql",
    method: ["GET", "POST", "OPTIONS"],
    handler: async (req, reply) => {
      const response = await yoga.handleNodeRequestAndResponse(req.raw, reply.raw, {
        req,
        reply,
      } as Record<string, unknown>);
      response.headers.forEach((value, key) => reply.header(key, value));
      reply.status(response.status);
      reply.send(response.body ? Buffer.from(await response.text()) : null);
      return reply;
    },
  });

  await registerRoutes(app);
  return app;
}

async function main() {
  const role = env.ROLE;
  log.info(`starting WorldSignal (role=${role}, llm=${hasOpenAI ? "openai" : "heuristic-fallback"})`);

  // Workers + scheduler (role: all | worker)
  if (role === "all" || role === "worker") {
    await getBoss();
    await registerWorkers();
    startScheduler();
  }

  // HTTP API (role: all | api)
  if (role === "all" || role === "api") {
    const app = await buildApi();
    await app.listen({ port: env.PORT, host: env.HOST });
    log.info(`API on http://${env.HOST}:${env.PORT}  (GraphQL: /graphql, REST: /v1/*)`);
  }

  const shutdown = async (sig: string) => {
    log.info(`${sig} received, shutting down`);
    stopScheduler();
    await stopBoss();
    await prisma.$disconnect();
    process.exit(0);
  };
  process.on("SIGINT", () => void shutdown("SIGINT"));
  process.on("SIGTERM", () => void shutdown("SIGTERM"));
}

main().catch((err) => {
  log.error("fatal", err);
  process.exit(1);
});
