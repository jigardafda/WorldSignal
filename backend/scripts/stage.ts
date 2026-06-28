// Differential test harness driver: runs a single pipeline stage of the legacy
// TypeScript backend against the database and prints its result as JSON. Used by
// the Go `parity` package to shadow-run stages and compare persisted rows.
//
// Usage: STAGE=<name> ARG='<json>' node --import tsx scripts/stage.ts
import { prisma } from "../src/db/prisma.js";
import { normalizeRawItem } from "../src/pipeline/normalize.js";
import { clusterArticle } from "../src/pipeline/cluster.js";
import { enrichSignal } from "../src/pipeline/enrichSignal.js";
import { matchSubscriptions, sendDelivery } from "../src/pipeline/deliver.js";
import { fetchSource } from "../src/pipeline/fetchSource.js";
import { fetchRssSource } from "../src/ingestion/rss.js";
import { enrichArticle } from "../src/llm/enrich.js";

async function main() {
  const stage = process.env.STAGE ?? "";
  const arg = JSON.parse(process.env.ARG ?? "{}");
  let result: unknown;
  switch (stage) {
    case "normalize":
      result = await normalizeRawItem(arg.rawItemId);
      break;
    case "cluster":
      result = await clusterArticle(arg.articleId);
      break;
    case "enrich":
      await enrichSignal(arg.signalId);
      result = "ok";
      break;
    case "match":
      result = await matchSubscriptions(arg.signalId);
      break;
    case "send":
      await sendDelivery(arg.deliveryId, arg.isFinal);
      result = "ok";
      break;
    case "fetch":
      result = await fetchSource(arg.sourceId);
      break;
    case "rss":
      result = await fetchRssSource(arg.url);
      break;
    case "enrichArticle":
      result = await enrichArticle(arg);
      break;
    case "pipeline": {
      // Full chain for one source, mirroring workers.ts (POLLING deliveries).
      const rawIds = await fetchSource(arg.sourceId);
      for (const rid of rawIds) {
        const articleId = await normalizeRawItem(rid);
        if (!articleId) continue;
        const cluster = await clusterArticle(articleId);
        if (!cluster) continue;
        await enrichSignal(cluster.signalId);
        const deliveryIds = await matchSubscriptions(cluster.signalId);
        for (const did of deliveryIds) await sendDelivery(did, false);
      }
      result = "ok";
      break;
    }
    default:
      throw new Error(`unknown stage: ${stage}`);
  }
  process.stdout.write(JSON.stringify(result ?? null));
}

main()
  .catch((e) => {
    process.stderr.write(String(e?.stack ?? e));
    process.exit(1);
  })
  .finally(() => prisma.$disconnect());
