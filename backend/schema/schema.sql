-- WorldSignal base content schema (Prisma-owned tables + enums).
-- Auth (User/Session/Team/TeamMember), Source metadata extensions,
-- SourceValidationLog, LLMKey, AuditLog and ws_jobs are created idempotently
-- by the Go server on boot (db.MigrateAuth / db.MigrateContent / queue.Migrate).
-- Apply this once to bootstrap a fresh database (CI, new dev setup):
--   psql "$DATABASE_URL" -f backend/schema/schema.sql

CREATE TYPE public."DeliveryChannel" AS ENUM (
    'WEBHOOK',
    'POLLING',
    'EMAIL'
);
CREATE TYPE public."DeliveryStatus" AS ENUM (
    'PENDING',
    'SENT',
    'FAILED',
    'RETRYING',
    'DEAD_LETTERED'
);
CREATE TYPE public."RawItemStatus" AS ENUM (
    'PENDING',
    'PARSED',
    'FAILED',
    'DUPLICATE'
);
CREATE TYPE public."Severity" AS ENUM (
    'LOW',
    'MEDIUM',
    'HIGH',
    'CRITICAL'
);
CREATE TYPE public."SignalArticleRelation" AS ENUM (
    'PRIMARY',
    'DUPLICATE',
    'SUPPORTING',
    'CONTRADICTING',
    'UPDATE'
);
CREATE TYPE public."SignalStatus" AS ENUM (
    'UNVERIFIED',
    'DEVELOPING',
    'CONFIRMED',
    'DISPUTED',
    'CORRECTED',
    'RETRACTED',
    'RESOLVED'
);
CREATE TYPE public."SourceType" AS ENUM (
    'RSS',
    'ATOM',
    'SITEMAP',
    'WEB_PAGE',
    'API',
    'WEBHOOK',
    'MANUAL'
);
CREATE TYPE public."SubscriberStatus" AS ENUM (
    'ACTIVE',
    'SUSPENDED',
    'DELETED'
);
CREATE TABLE public."Article" (
    id text NOT NULL,
    "rawItemId" text,
    "sourceId" text NOT NULL,
    "canonicalUrl" text,
    title text NOT NULL,
    body text,
    summary text,
    author text,
    language text DEFAULT 'en'::text,
    country text,
    "publishedAt" timestamp(3) without time zone,
    "fetchedAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "contentHash" text,
    "tokenSet" text,
    metadata jsonb
);
CREATE TABLE public."DeliveryEvent" (
    id text NOT NULL,
    "subscriptionId" text NOT NULL,
    "signalId" text NOT NULL,
    channel public."DeliveryChannel" NOT NULL,
    status public."DeliveryStatus" DEFAULT 'PENDING'::public."DeliveryStatus" NOT NULL,
    payload jsonb NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    "deliveredAt" timestamp(3) without time zone,
    "failedAt" timestamp(3) without time zone,
    "errorMessage" text,
    "createdAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE public."RawItem" (
    id text NOT NULL,
    "sourceId" text NOT NULL,
    "rawUrl" text,
    "sourceGuid" text,
    "rawTitle" text,
    "rawContent" text,
    "rawPayload" jsonb,
    "contentHash" text,
    "publishedAt" timestamp(3) without time zone,
    "fetchedAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    status public."RawItemStatus" DEFAULT 'PENDING'::public."RawItemStatus" NOT NULL
);
CREATE TABLE public."Signal" (
    id text NOT NULL,
    title text NOT NULL,
    summary text NOT NULL,
    "whatHappened" text,
    "whyItMatters" text,
    status public."SignalStatus" DEFAULT 'UNVERIFIED'::public."SignalStatus" NOT NULL,
    severity public."Severity" DEFAULT 'MEDIUM'::public."Severity" NOT NULL,
    confidence double precision DEFAULT 0.5 NOT NULL,
    "eventType" text,
    country text,
    region text,
    "firstSeenAt" timestamp(3) without time zone NOT NULL,
    "lastSeenAt" timestamp(3) without time zone NOT NULL,
    "publishedAt" timestamp(3) without time zone,
    "createdAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updatedAt" timestamp(3) without time zone NOT NULL,
    "sourceCount" integer DEFAULT 1 NOT NULL,
    metadata jsonb,
    city text,
    locality text,
    "geoScope" text,
    sentiment text,
    "sentimentScore" double precision,
    influence text,
    relevance double precision,
    language text
);
CREATE TABLE public."SignalAttribute" (
    "signalId" text NOT NULL,
    key text NOT NULL,
    "valueCode" text DEFAULT ''::text NOT NULL,
    "valueText" text DEFAULT ''::text NOT NULL,
    "valueNum" double precision,
    confidence double precision DEFAULT 1 NOT NULL
);
CREATE TABLE public."SignalArticle" (
    "signalId" text NOT NULL,
    "articleId" text NOT NULL,
    "relationType" public."SignalArticleRelation" DEFAULT 'PRIMARY'::public."SignalArticleRelation" NOT NULL,
    "similarityScore" double precision,
    "addedAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE public."SignalTag" (
    "signalId" text NOT NULL,
    "tagId" text NOT NULL,
    confidence double precision DEFAULT 0.5 NOT NULL
);
CREATE TABLE public."Source" (
    id text NOT NULL,
    name text NOT NULL,
    type public."SourceType" DEFAULT 'RSS'::public."SourceType" NOT NULL,
    url text NOT NULL,
    country text,
    region text,
    language text DEFAULT 'en'::text,
    category text,
    priority integer DEFAULT 5 NOT NULL,
    credibility double precision DEFAULT 0.5 NOT NULL,
    "crawlFrequency" integer DEFAULT 900 NOT NULL,
    "parserType" text DEFAULT 'rss'::text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    config jsonb,
    "lastFetchedAt" timestamp(3) without time zone,
    "lastSuccessAt" timestamp(3) without time zone,
    "lastFailureAt" timestamp(3) without time zone,
    "failureCount" integer DEFAULT 0 NOT NULL,
    "createdAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updatedAt" timestamp(3) without time zone NOT NULL,
    "websiteUrl" text,
    languages text[] DEFAULT '{}'::text[] NOT NULL,
    "geographicScope" text,
    industry text,
    subcategory text,
    publisher text,
    "orgType" text,
    "sourceType" text,
    "officialFeed" boolean DEFAULT false NOT NULL,
    "contentType" text,
    "updateFrequency" text,
    tags text[] DEFAULT '{}'::text[] NOT NULL,
    "biasRating" text,
    "healthScore" integer,
    "validationStatus" text DEFAULT 'PENDING'::text NOT NULL,
    "lastValidatedAt" timestamp with time zone,
    "lastValidationError" text,
    "avgResponseMs" integer,
    metadata jsonb,
    "cooldownUntil" timestamp with time zone
);
CREATE TABLE public."Subscriber" (
    id text NOT NULL,
    name text NOT NULL,
    status public."SubscriberStatus" DEFAULT 'ACTIVE'::public."SubscriberStatus" NOT NULL,
    "createdAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE public."Subscription" (
    id text NOT NULL,
    "subscriberId" text NOT NULL,
    name text NOT NULL,
    channel public."DeliveryChannel" DEFAULT 'WEBHOOK'::public."DeliveryChannel" NOT NULL,
    filter jsonb NOT NULL,
    config jsonb NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    "createdAt" timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE public."TaxonomyTag" (
    id text NOT NULL,
    code text NOT NULL,
    label text NOT NULL,
    description text,
    "parentId" text,
    aliases text[],
    active boolean DEFAULT true NOT NULL
);
ALTER TABLE ONLY public."Article"
    ADD CONSTRAINT "Article_pkey" PRIMARY KEY (id);
ALTER TABLE ONLY public."DeliveryEvent"
    ADD CONSTRAINT "DeliveryEvent_pkey" PRIMARY KEY (id);
ALTER TABLE ONLY public."RawItem"
    ADD CONSTRAINT "RawItem_pkey" PRIMARY KEY (id);
ALTER TABLE ONLY public."SignalAttribute"
    ADD CONSTRAINT "SignalAttribute_pkey" PRIMARY KEY ("signalId", key, "valueCode", "valueText");
ALTER TABLE ONLY public."SignalArticle"
    ADD CONSTRAINT "SignalArticle_pkey" PRIMARY KEY ("signalId", "articleId");
ALTER TABLE ONLY public."SignalTag"
    ADD CONSTRAINT "SignalTag_pkey" PRIMARY KEY ("signalId", "tagId");
ALTER TABLE ONLY public."Signal"
    ADD CONSTRAINT "Signal_pkey" PRIMARY KEY (id);
ALTER TABLE ONLY public."Source"
    ADD CONSTRAINT "Source_pkey" PRIMARY KEY (id);
ALTER TABLE ONLY public."Subscriber"
    ADD CONSTRAINT "Subscriber_pkey" PRIMARY KEY (id);
ALTER TABLE ONLY public."Subscription"
    ADD CONSTRAINT "Subscription_pkey" PRIMARY KEY (id);
ALTER TABLE ONLY public."TaxonomyTag"
    ADD CONSTRAINT "TaxonomyTag_pkey" PRIMARY KEY (id);
CREATE INDEX "Article_canonicalUrl_idx" ON public."Article" USING btree ("canonicalUrl");
CREATE INDEX "Article_contentHash_idx" ON public."Article" USING btree ("contentHash");
CREATE INDEX "Article_fetchedAt_idx" ON public."Article" USING btree ("fetchedAt" DESC);
CREATE INDEX "Article_publishedAt_idx" ON public."Article" USING btree ("publishedAt");
CREATE UNIQUE INDEX "Article_rawItemId_key" ON public."Article" USING btree ("rawItemId");
CREATE INDEX "Article_sourceId_idx" ON public."Article" USING btree ("sourceId");
CREATE INDEX "DeliveryEvent_createdAt_idx" ON public."DeliveryEvent" USING btree ("createdAt" DESC);
CREATE INDEX "DeliveryEvent_status_idx" ON public."DeliveryEvent" USING btree (status);
CREATE UNIQUE INDEX "DeliveryEvent_subscriptionId_signalId_key" ON public."DeliveryEvent" USING btree ("subscriptionId", "signalId");
CREATE INDEX "RawItem_contentHash_idx" ON public."RawItem" USING btree ("contentHash");
CREATE INDEX "RawItem_fetchedAt_idx" ON public."RawItem" USING btree ("fetchedAt" DESC);
CREATE INDEX "RawItem_publishedAt_idx" ON public."RawItem" USING btree ("publishedAt");
CREATE UNIQUE INDEX "RawItem_sourceId_sourceGuid_key" ON public."RawItem" USING btree ("sourceId", "sourceGuid");
CREATE INDEX "RawItem_status_idx" ON public."RawItem" USING btree (status);
CREATE INDEX "SignalArticle_articleId_idx" ON public."SignalArticle" USING btree ("articleId");
CREATE INDEX "SignalTag_tagId_idx" ON public."SignalTag" USING btree ("tagId");
CREATE INDEX "Signal_confidence_idx" ON public."Signal" USING btree (confidence);
CREATE INDEX "Signal_country_idx" ON public."Signal" USING btree (country);
CREATE INDEX "Signal_region_idx" ON public."Signal" USING btree (region);
CREATE INDEX "Signal_geoScope_idx" ON public."Signal" USING btree ("geoScope");
CREATE INDEX "Signal_sentiment_idx" ON public."Signal" USING btree (sentiment);
CREATE INDEX "Signal_influence_idx" ON public."Signal" USING btree (influence);
CREATE INDEX "SignalAttribute_key_value_idx" ON public."SignalAttribute" USING btree (key, "valueCode");
CREATE INDEX "SignalAttribute_signal_idx" ON public."SignalAttribute" USING btree ("signalId");
CREATE INDEX "Signal_lastSeenAt_idx" ON public."Signal" USING btree ("lastSeenAt");
CREATE INDEX "Signal_severity_idx" ON public."Signal" USING btree (severity);
CREATE INDEX "Signal_status_idx" ON public."Signal" USING btree (status);
CREATE INDEX "Source_country_idx" ON public."Source" USING btree (country);
CREATE INDEX "Source_enabled_cooldown_idx" ON public."Source" USING btree (enabled, "cooldownUntil", priority);
CREATE INDEX "Source_enabled_priority_idx" ON public."Source" USING btree (enabled, priority);
CREATE INDEX "Source_industry_idx" ON public."Source" USING btree (industry);
CREATE INDEX "Source_language_idx" ON public."Source" USING btree (language);
CREATE INDEX "Source_region_idx" ON public."Source" USING btree (region);
CREATE INDEX "Source_scope_idx" ON public."Source" USING btree ("geographicScope");
CREATE INDEX "Source_tags_idx" ON public."Source" USING gin (tags);
CREATE INDEX "Source_type_idx" ON public."Source" USING btree (type);
CREATE UNIQUE INDEX "Source_url_key" ON public."Source" USING btree (url);
CREATE INDEX "Source_validation_idx" ON public."Source" USING btree ("validationStatus");
CREATE INDEX "Subscription_createdAt_idx" ON public."Subscription" USING btree ("createdAt" DESC);
CREATE UNIQUE INDEX "TaxonomyTag_code_key" ON public."TaxonomyTag" USING btree (code);
ALTER TABLE ONLY public."Article"
    ADD CONSTRAINT "Article_rawItemId_fkey" FOREIGN KEY ("rawItemId") REFERENCES public."RawItem"(id) ON UPDATE CASCADE ON DELETE SET NULL;
ALTER TABLE ONLY public."Article"
    ADD CONSTRAINT "Article_sourceId_fkey" FOREIGN KEY ("sourceId") REFERENCES public."Source"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."DeliveryEvent"
    ADD CONSTRAINT "DeliveryEvent_signalId_fkey" FOREIGN KEY ("signalId") REFERENCES public."Signal"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."DeliveryEvent"
    ADD CONSTRAINT "DeliveryEvent_subscriptionId_fkey" FOREIGN KEY ("subscriptionId") REFERENCES public."Subscription"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."RawItem"
    ADD CONSTRAINT "RawItem_sourceId_fkey" FOREIGN KEY ("sourceId") REFERENCES public."Source"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."SignalAttribute"
    ADD CONSTRAINT "SignalAttribute_signalId_fkey" FOREIGN KEY ("signalId") REFERENCES public."Signal"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."SignalArticle"
    ADD CONSTRAINT "SignalArticle_articleId_fkey" FOREIGN KEY ("articleId") REFERENCES public."Article"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."SignalArticle"
    ADD CONSTRAINT "SignalArticle_signalId_fkey" FOREIGN KEY ("signalId") REFERENCES public."Signal"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."SignalTag"
    ADD CONSTRAINT "SignalTag_signalId_fkey" FOREIGN KEY ("signalId") REFERENCES public."Signal"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."SignalTag"
    ADD CONSTRAINT "SignalTag_tagId_fkey" FOREIGN KEY ("tagId") REFERENCES public."TaxonomyTag"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."Subscription"
    ADD CONSTRAINT "Subscription_subscriberId_fkey" FOREIGN KEY ("subscriberId") REFERENCES public."Subscriber"(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public."TaxonomyTag"
    ADD CONSTRAINT "TaxonomyTag_parentId_fkey" FOREIGN KEY ("parentId") REFERENCES public."TaxonomyTag"(id) ON UPDATE CASCADE ON DELETE SET NULL;
