import { createSchema, createYoga } from "graphql-yoga";
import { prisma } from "../db/prisma.js";
import { TAXONOMY } from "../taxonomy/taxonomy.js";
import { enqueueFetchSource } from "../jobs/workers.js";

const typeDefs = /* GraphQL */ `
  scalar DateTime
  scalar JSON

  type TaxonomyTag {
    code: String!
    label: String!
  }

  type SignalTag {
    code: String!
    confidence: Float!
  }

  type ArticleSource {
    publisher: String!
    url: String
    publishedAt: DateTime
  }

  type Signal {
    id: ID!
    title: String!
    summary: String!
    whatHappened: String
    whyItMatters: String
    status: String!
    severity: String!
    confidence: Float!
    eventType: String
    country: String
    sourceCount: Int!
    firstSeenAt: DateTime!
    lastSeenAt: DateTime!
    tags: [SignalTag!]!
    sources: [ArticleSource!]!
  }

  type Source {
    id: ID!
    name: String!
    type: String!
    url: String!
    country: String
    priority: Int!
    credibility: Float!
    enabled: Boolean!
    lastSuccessAt: DateTime
    lastFailureAt: DateTime
    failureCount: Int!
  }

  input SignalFilter {
    tags: [String!]
    country: String
    minConfidence: Float
    status: String
    search: String
  }

  type Query {
    signals(filter: SignalFilter, limit: Int = 50, offset: Int = 0): [Signal!]!
    signal(id: ID!): Signal
    sources: [Source!]!
    subscriptions: [Subscription!]!
    taxonomy: JSON!
    stats: JSON!
  }

  input CreateSourceInput {
    name: String!
    url: String!
    type: String
    country: String
    priority: Int
    crawlFrequency: Int
    credibility: Float
  }

  type Subscriber {
    id: ID!
    name: String!
  }

  type Subscription {
    id: ID!
    name: String!
    channel: String!
    enabled: Boolean!
    filter: JSON!
    config: JSON!
    createdAt: DateTime!
  }

  input CreateSubscriptionInput {
    name: String!
    channel: String
    filter: JSON
    config: JSON
  }

  type Mutation {
    createSource(input: CreateSourceInput!): Source!
    setSourceEnabled(id: ID!, enabled: Boolean!): Source!
    triggerFetch(id: ID!): Boolean!
    createSubscription(input: CreateSubscriptionInput!): Subscription!
  }
`;

function tagWhere(filter: any) {
  const where: any = {};
  if (filter?.country) where.country = filter.country;
  if (filter?.status) where.status = filter.status;
  if (filter?.minConfidence != null) where.confidence = { gte: filter.minConfidence };
  if (filter?.search) {
    where.OR = [
      { title: { contains: filter.search, mode: "insensitive" } },
      { summary: { contains: filter.search, mode: "insensitive" } },
    ];
  }
  if (filter?.tags?.length) {
    where.tags = { some: { tag: { code: { in: filter.tags } } } };
  }
  return where;
}

async function serializeSignal(s: any) {
  return {
    ...s,
    tags: s.tags.map((t: any) => ({ code: t.tag.code, confidence: t.confidence })),
    sources: s.articles.map((a: any) => ({
      publisher: a.article.source.name,
      url: a.article.canonicalUrl,
      publishedAt: a.article.publishedAt,
    })),
  };
}

const resolvers = {
  Query: {
    signals: async (_: unknown, args: any) => {
      const rows = await prisma.signal.findMany({
        where: tagWhere(args.filter),
        orderBy: { lastSeenAt: "desc" },
        take: Math.min(args.limit ?? 50, 200),
        skip: args.offset ?? 0,
        include: { tags: { include: { tag: true } }, articles: { include: { article: { include: { source: true } } } } },
      });
      return Promise.all(rows.map(serializeSignal));
    },
    signal: async (_: unknown, args: { id: string }) => {
      const s = await prisma.signal.findUnique({
        where: { id: args.id },
        include: { tags: { include: { tag: true } }, articles: { include: { article: { include: { source: true } } } } },
      });
      return s ? serializeSignal(s) : null;
    },
    sources: () => prisma.source.findMany({ orderBy: [{ priority: "asc" }, { name: "asc" }] }),
    subscriptions: () => prisma.subscription.findMany({ orderBy: { createdAt: "desc" } }),
    taxonomy: () => TAXONOMY,
    stats: async () => {
      const [sources, articles, signals, deliveries] = await Promise.all([
        prisma.source.count(),
        prisma.article.count(),
        prisma.signal.count(),
        prisma.deliveryEvent.count({ where: { status: "SENT" } }),
      ]);
      return { sources, articles, signals, deliveriesSent: deliveries };
    },
  },
  Mutation: {
    createSource: (_: unknown, args: { input: any }) =>
      prisma.source.create({
        data: {
          name: args.input.name,
          url: args.input.url,
          type: args.input.type ?? "RSS",
          country: args.input.country,
          priority: args.input.priority ?? 5,
          crawlFrequency: args.input.crawlFrequency ?? 900,
          credibility: args.input.credibility ?? 0.5,
        },
      }),
    setSourceEnabled: (_: unknown, args: { id: string; enabled: boolean }) =>
      prisma.source.update({ where: { id: args.id }, data: { enabled: args.enabled } }),
    triggerFetch: async (_: unknown, args: { id: string }) => {
      await enqueueFetchSource(args.id);
      return true;
    },
    createSubscription: async (_: unknown, args: { input: any }) => {
      const subscriber = await prisma.subscriber.upsert({
        where: { id: "__default__" },
        update: {},
        create: { id: "__default__", name: "Default Subscriber" },
      });
      return prisma.subscription.create({
        data: {
          subscriberId: subscriber.id,
          name: args.input.name,
          channel: args.input.channel ?? "WEBHOOK",
          filter: args.input.filter ?? {},
          config: args.input.config ?? {},
        },
      });
    },
  },
};

export const yoga = createYoga({
  schema: createSchema({ typeDefs, resolvers }),
  graphqlEndpoint: "/graphql",
  landingPage: false,
});
