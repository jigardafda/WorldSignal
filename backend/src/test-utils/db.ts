import "./env.js";
import { prisma } from "../db/prisma.js";
import { TAXONOMY, type TaxonomyNode } from "../taxonomy/taxonomy.js";

const TABLES = [
  "DeliveryEvent",
  "Subscription",
  "Subscriber",
  "SignalTag",
  "SignalArticle",
  "Signal",
  "Article",
  "RawItem",
  "Source",
  "TaxonomyTag",
];

/** Wipe all application tables (leaves the pg-boss schema untouched). */
export async function resetDb(): Promise<void> {
  const list = TABLES.map((t) => `"${t}"`).join(", ");
  await prisma.$executeRawUnsafe(`TRUNCATE TABLE ${list} RESTART IDENTITY CASCADE`);
}

async function upsertTag(node: TaxonomyNode, parentId: string | null): Promise<void> {
  const tag = await prisma.taxonomyTag.upsert({
    where: { code: node.code },
    update: { label: node.label, parentId },
    create: { code: node.code, label: node.label, parentId, aliases: node.aliases ?? [] },
  });
  for (const child of node.children ?? []) await upsertTag(child, tag.id);
}

export async function seedTaxonomy(): Promise<void> {
  for (const domain of TAXONOMY) await upsertTag(domain, null);
}
