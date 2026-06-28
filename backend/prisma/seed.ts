import { PrismaClient } from "@prisma/client";
import { TAXONOMY, type TaxonomyNode } from "../src/taxonomy/taxonomy.js";

const prisma = new PrismaClient();

async function upsertTag(node: TaxonomyNode, parentId: string | null) {
  const tag = await prisma.taxonomyTag.upsert({
    where: { code: node.code },
    update: { label: node.label, parentId, aliases: node.aliases ?? [] },
    create: { code: node.code, label: node.label, parentId, aliases: node.aliases ?? [] },
  });
  for (const child of node.children ?? []) {
    await upsertTag(child, tag.id);
  }
}

// A small set of high-quality, public RSS feeds across categories/regions.
const SOURCES = [
  { name: "BBC World", url: "https://feeds.bbci.co.uk/news/world/rss.xml", country: "GB", category: "general", priority: 1, credibility: 0.9, crawlFrequency: 300 },
  { name: "Reuters Top News", url: "https://www.reutersagency.com/feed/?best-topics=top-news&post_type=best", country: "US", category: "general", priority: 1, credibility: 0.92, crawlFrequency: 300 },
  { name: "NASA Breaking News", url: "https://www.nasa.gov/news-release/feed/", country: "US", category: "science", priority: 2, credibility: 0.95, crawlFrequency: 1800 },
  { name: "USGS Earthquakes (M2.5+)", url: "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/2.5_day.atom", type: "ATOM", country: "US", category: "disaster", priority: 0, credibility: 0.99, crawlFrequency: 120 },
  { name: "The Verge", url: "https://www.theverge.com/rss/index.xml", country: "US", category: "technology", priority: 2, credibility: 0.75, crawlFrequency: 600 },
  { name: "Hacker News Front Page", url: "https://hnrss.org/frontpage", country: "US", category: "technology", priority: 3, credibility: 0.6, crawlFrequency: 900 },
  { name: "The Guardian World", url: "https://www.theguardian.com/world/rss", country: "GB", category: "general", priority: 1, credibility: 0.85, crawlFrequency: 300 },
];

async function main() {
  console.log("Seeding taxonomy…");
  for (const domain of TAXONOMY) await upsertTag(domain, null);
  const tagCount = await prisma.taxonomyTag.count();
  console.log(`  ${tagCount} taxonomy tags`);

  console.log("Seeding sources…");
  for (const s of SOURCES) {
    await prisma.source.upsert({
      where: { url: s.url },
      update: {},
      create: {
        name: s.name,
        url: s.url,
        type: (s.type as any) ?? "RSS",
        country: s.country,
        category: s.category,
        priority: s.priority,
        credibility: s.credibility,
        crawlFrequency: s.crawlFrequency,
      },
    });
  }
  const srcCount = await prisma.source.count();
  console.log(`  ${srcCount} sources`);

  // A demo polling subscription so the delivery path has a subscriber out of the box.
  const subscriber = await prisma.subscriber.upsert({
    where: { id: "__default__" },
    update: {},
    create: { id: "__default__", name: "Default Subscriber" },
  });
  const existing = await prisma.subscription.findFirst({ where: { name: "All signals (polling)" } });
  if (!existing) {
    await prisma.subscription.create({
      data: {
        subscriberId: subscriber.id,
        name: "All signals (polling)",
        channel: "POLLING",
        filter: {},
        config: {},
      },
    });
  }

  console.log("Done.");
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  })
  .finally(() => prisma.$disconnect());
