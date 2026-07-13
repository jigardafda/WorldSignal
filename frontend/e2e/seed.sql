-- Deterministic seed for end-to-end tests (applied to worldsignal_e2e).
TRUNCATE TABLE "DeliveryEvent","Subscription","SignalTag","SignalArticle","Signal","Article","RawItem","Source","TaxonomyTag" RESTART IDENTITY CASCADE;
-- Include Account + ApiKey so accounts/keys created during a test run don't
-- persist and collide (e.g. a duplicate slug) on the next run.
TRUNCATE TABLE "Session","TeamMember","Team","ApiKey","User","Account" RESTART IDENTITY CASCADE;

-- The default account is normally created by the Go migration at boot; re-seed
-- it here since the TRUNCATE above wiped it (the migration only runs once).
INSERT INTO "Account" ("id","name","slug") VALUES ('acct_default','Default Account','default')
  ON CONFLICT ("id") DO NOTHING;

-- Admin account used to log in during e2e. Password is "admin12345" (bcrypt cost 10).
INSERT INTO "User" ("id","email","name","passwordHash","role","status","createdAt","updatedAt")
VALUES ('e_admin','admin@worldsignal.local','E2E Admin','$2a$10$nRmQ72FEbXqLz461pnCWX.OUTUqgAssob6YLoJfMHgd0GAb6g/MRa','ADMIN','ACTIVE',now(),now());

INSERT INTO "TaxonomyTag" ("id","code","label","active") VALUES
  ('t_dis','DISASTER','Disaster',true),
  ('t_eq','DISASTER.EARTHQUAKE','Earthquake',true);

INSERT INTO "Source" ("id","name","type","url","country","priority","credibility","crawlFrequency","parserType","enabled","failureCount","createdAt","updatedAt")
VALUES ('e_src','BBC World','RSS','https://bbc.example/feed','GB',1,0.9,300,'rss',true,0,now(),now());

INSERT INTO "Signal" ("id","title","summary","whatHappened","whyItMatters","status","severity","confidence","eventType","country","sourceCount","firstSeenAt","lastSeenAt","publishedAt","createdAt","updatedAt")
VALUES ('e_sig','Major earthquake strikes region','A strong earthquake struck the region.','A strong earthquake struck.','Thousands affected.','CONFIRMED','HIGH',0.82,'DISASTER.EARTHQUAKE','US',3,now(),now(),now(),now(),now());

INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
VALUES ('e_art','e_src','https://bbc.example/quake','Major earthquake','A strong earthquake struck the region.','A strong earthquake struck the region.',now(),'eh','earthquake region strong struck');

INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore") VALUES ('e_sig','e_art','PRIMARY',1);
INSERT INTO "SignalTag" ("signalId","tagId","confidence") VALUES ('e_sig','t_eq',0.9);

-- A tenant account + an account-scoped (customer console) user. The Account
-- table is created by the Go server's migration on boot; reuse the admin bcrypt
-- hash so the tenant logs in with the same password ("admin12345").
INSERT INTO "Account" ("id","name","slug","plan","status") VALUES ('e_acct','Tenant Inc','tenant-inc','PRO','ACTIVE')
  ON CONFLICT ("id") DO NOTHING;
INSERT INTO "User" ("id","email","name","passwordHash","role","status","accountId","createdAt","updatedAt")
VALUES ('e_tenant','tenant@acme.test','Acme User','$2a$10$nRmQ72FEbXqLz461pnCWX.OUTUqgAssob6YLoJfMHgd0GAb6g/MRa','ADMIN','ACTIVE','e_acct',now(),now());

INSERT INTO "Subscription" ("id","accountId","name","channel","filter","config","enabled","createdAt")
VALUES ('e_sub','e_acct','All signals (polling)','POLLING','{}','{}',true,now());
