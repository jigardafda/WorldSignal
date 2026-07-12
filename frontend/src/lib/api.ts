// Typed GraphQL operations for the whole console.
import { gql } from "./graphql";

// ---- shared types ----
export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  permissions?: string[];
}
export interface SignalTag { code: string; confidence: number }
export interface SignalSource { publisher: string; url: string | null; publishedAt: string | null }
export interface SignalAttribute { key: string; valueCode: string; valueText: string; valueNum: number | null; confidence: number }
export interface LiveSignal { id: string; title: string; country?: string | null; region?: string | null; city?: string | null; severity: string; eventType?: string | null; lastSeenAt: string; sourceCount?: number; sentiment?: string | null; influence?: string | null; relevance?: number | null }
export interface AttributeValue { code: string; label: string }
export interface AttributeDefinition { key: string; label: string; kind: string; description: string; values: AttributeValue[] }
export interface Signal {
  id: string; title: string; summary: string;
  whatHappened?: string | null; whyItMatters?: string | null;
  status: string; severity: string; confidence: number;
  eventType?: string | null; country?: string | null; sourceCount: number;
  firstSeenAt: string; lastSeenAt: string;
  tags: SignalTag[]; sources: SignalSource[];
  // Deep-enrichment attributes.
  region?: string | null; city?: string | null; locality?: string | null; geoScope?: string | null;
  sentiment?: string | null; sentimentScore?: number | null; influence?: string | null; relevance?: number | null;
  language?: string | null; translated?: boolean;
  originalTitle?: string | null; originalSummary?: string | null;
  attributes?: SignalAttribute[];
}
export interface Source {
  id: string; name: string; type: string; url: string;
  country?: string | null; region?: string | null; language?: string | null; category?: string | null;
  priority: number; credibility: number; crawlFrequency: number; parserType: string;
  enabled: boolean; failureCount: number;
  lastSuccessAt?: string | null; lastFailureAt?: string | null; lastFetchedAt?: string | null;
  createdAt?: string; updatedAt?: string;
  // Rich metadata (global-source expansion).
  websiteUrl?: string | null; languages?: string[]; geographicScope?: string | null;
  industry?: string | null; subcategory?: string | null; publisher?: string | null;
  orgType?: string | null; sourceType?: string | null; officialFeed?: boolean;
  contentType?: string | null; updateFrequency?: string | null; biasRating?: string | null;
  tags?: string[]; healthScore?: number | null; validationStatus?: string;
  lastValidatedAt?: string | null; lastValidationError?: string | null; avgResponseMs?: number | null;
  cooldownUntil?: string | null;
  metadata?: unknown; validationLogs?: ValidationLog[];
}
export interface ValidationLog {
  id: string; checkedAt: string; ok: boolean; httpStatus?: number | null;
  responseMs?: number | null; itemCount?: number | null; newestItemAt?: string | null;
  redirectedTo?: string | null; error?: string | null;
}
export interface SourceFilter {
  search?: string; country?: string; region?: string; language?: string; scope?: string;
  industry?: string; orgType?: string; sourceType?: string; validationStatus?: string;
  tag?: string; enabled?: boolean; pollStatus?: string;
}
export interface SourceCoverage {
  byRegion: Bucket[]; byScope: Bucket[]; byOrgType: Bucket[]; byValidation: Bucket[];
  byIndustry: Bucket[]; byCountry: Bucket[]; bySourceType: Bucket[]; byLanguage: Bucket[];
}
export interface Page<T> { items: T[]; total: number }
export interface ArticleRow {
  id: string; title: string; canonicalUrl: string | null; summary: string | null;
  publishedAt: string | null; fetchedAt: string; sourceId: string; sourceName: string; signalCount: number;
}
export interface ArticleDetail extends ArticleRow {
  body: string | null; author: string | null; language: string | null; country: string | null;
  contentHash: string | null; tokenSet: string | null;
  signals: { id: string; title: string; relationType: string; similarityScore: number | null }[];
}
export interface LLMKey {
  id: string; provider: string; label: string; keyLast4: string; model: string | null;
  isActive: boolean; status: string; lastTestedAt: string | null; lastError: string | null;
  createdBy: string | null; createdAt: string; updatedAt: string;
}
export interface LLMStatus {
  provider: string; enabled: boolean; source: string; model: string;
  hasSystemKey: boolean; activeLabel: string | null;
}
export interface LLMTestResult { ok: boolean; status: string; error?: string | null }
export interface AuditLog {
  id: string; actorId: string | null; actorEmail: string | null; actorRole: string | null;
  action: string; targetType: string | null; targetId: string | null;
  metadata: unknown; createdAt: string;
}
export interface AuditFilter { actor?: string; action?: string; targetType?: string; search?: string }
export interface Country {
  code: string; name: string; flag: string; currency: string;
  capital: string; capitalLat: number; capitalLng: number;
}
export interface RawItemRow {
  id: string; sourceId: string; sourceName: string; sourceGuid: string | null;
  rawUrl: string | null; rawTitle: string | null; status: string;
  publishedAt: string | null; fetchedAt: string;
}
export interface RawItemDetail extends RawItemRow {
  rawContent: string | null; contentHash: string | null; rawPayload: unknown;
}
export interface DeliveryRow {
  id: string; subscriptionId: string; subscriptionName: string; channel: string;
  signalId: string; signalTitle: string; status: string; attempts: number;
  createdAt: string; deliveredAt: string | null; failedAt: string | null; errorMessage: string | null;
}
export interface DeliveryDetail extends DeliveryRow { payload: unknown }
export interface Subscription {
  id: string; subscriberId?: string; name: string; channel: string; enabled: boolean;
  filter: unknown; config: unknown; createdAt: string;
}
export interface EmailConnector {
  id: string; name: string; provider: string; host: string; port: number; security: string;
  username: string; secretLast4: string; fromEmail: string; fromName: string;
  isActive: boolean; enabled: boolean; status: string;
  lastTestedAt: string | null; lastError: string | null; createdAt: string; updatedAt: string;
}
export interface EmailProvider {
  code: string; label: string; host: string; port: number; security: string;
  usernameHint: string; secretHint: string; help: string; docsAnchor: string; editable: boolean;
}
export interface ConnectorTestResult { ok: boolean; status?: string; error?: string | null }
export interface ApiKey {
  id: string; name: string; keyPrefix: string; scopes: string[];
  rateLimitPerMin: number; enabled: boolean; expiresAt: string | null;
  lastUsedAt: string | null; requestCount: number; createdBy: string | null; createdAt: string;
  key?: string; // the raw secret — only present in the createApiKey response
}
export interface Subscriber { id: string; name: string; status: string; createdAt: string; subscriptionCount: number }
export interface Account {
  id: string; name: string; slug: string; status: string; plan: string;
  createdAt: string; updatedAt: string;
}
export interface Entity { name: string; type: string; signalCount: number }
export interface EntityFilter { search?: string; type?: string }
export interface Bucket { key: string; count: number }
export interface Analytics {
  signalsBySeverity: Bucket[]; signalsByStatus: Bucket[]; signalsByEventType: Bucket[];
  signalsByCountry: Bucket[]; signalsOverTime: Bucket[];
  signalsBySentiment: Bucket[]; signalsByGeoScope: Bucket[]; topIndustries: Bucket[];
  topSources: { id: string; name: string; articleCount: number }[];
  deliveryStats: { total: number; sent: number; pending: number; retrying: number; failed: number; deadLettered: number };
  ingestionStats: { rawItems: number; parsed: number; duplicates: number; failed: number; articles: number };
}
export interface Job {
  id: string; queue: string; state: string; retryCount: number; retryLimit: number;
  createdAt: string; startedAt: string | null; completedAt: string | null; lastError: string | null;
}
export interface Stats { sources: number; articles: number; signals: number; deliveriesSent: number; deliveriesPending: number }
export interface TaxonomyNode { code: string; label: string; children?: TaxonomyNode[] }
export interface Team { id: string; name: string; createdAt: string; memberCount: number; members?: TeamMember[] }
export interface TeamMember { userId: string; email: string; name: string; role: string; addedAt: string }

const SIGNAL_FIELDS = `id title summary whatHappened whyItMatters status severity confidence eventType country sourceCount firstSeenAt lastSeenAt region city locality geoScope sentiment sentimentScore influence relevance language translated originalTitle originalSummary tags{code confidence} sources{publisher url publishedAt} attributes{key valueCode valueText valueNum confidence}`;
const SOURCE_LIST_FIELDS = `id name type url country region language languages category priority credibility enabled failureCount sourceType officialFeed industry publisher orgType geographicScope healthScore validationStatus tags lastSuccessAt lastFailureAt lastValidatedAt lastFetchedAt cooldownUntil`;
const SOURCE_FIELDS = `${SOURCE_LIST_FIELDS} crawlFrequency parserType subcategory websiteUrl contentType updateFrequency biasRating avgResponseMs lastValidationError lastFetchedAt createdAt updatedAt`;
const VALIDATION_LOG_FIELDS = `id checkedAt ok httpStatus responseMs itemCount newestItemAt redirectedTo error`;

// argList renders a SourceFilter as inline GraphQL arguments (skip undefined).
function sourceArgs(f: SourceFilter = {}, extra: Record<string, string | number> = {}): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === "") continue;
    parts.push(typeof v === "boolean" ? `${k}:${v}` : `${k}:${JSON.stringify(v)}`);
  }
  for (const [k, v] of Object.entries(extra)) parts.push(`${k}:${typeof v === "number" ? v : JSON.stringify(v)}`);
  return parts.length ? `(${parts.join(",")})` : "";
}
const USER_FIELDS = `id email name role status createdAt updatedAt`;

/** One ranked signal in a profile's "For You" feed. */
export interface FeedItem {
  id: string;
  title: string;
  summary: string;
  eventType: string;
  country: string;
  region: string;
  sentiment: string;
  influence: string;
  severity: string;
  ageHours: number;
  score: number;
  reasons: string[];
}

export type FeedbackAction = "OPEN" | "UP" | "DOWN";

/** A reason one interest was proposed, with its provenance. */
export interface DraftReason {
  key: string;
  why: string;
  origin: "doc" | "web" | "inferred";
}

/** An AI-proposed profile built from a document. */
export interface ProfileDraft {
  name: string;
  summary: string;
  minScore: number;
  minSeverity: string;
  source: "llm" | "heuristic";
  interests: Record<string, number>;
  reasons: DraftReason[];
}

export const api = {
  // auth
  login: (email: string, password: string) =>
    gql<{ login: { token: string; user: User } }>(
      `mutation($e:String!,$p:String!){login(email:$e,password:$p){token user{${USER_FIELDS} permissions}}}`,
      { e: email, p: password },
    ).then((d) => d.login),
  me: () => gql<{ me: User | null }>(`{me{${USER_FIELDS} permissions}}`).then((d) => d.me),
  logout: () => gql<{ logout: boolean }>(`mutation{logout}`).then((d) => d.logout),
  changePassword: (oldPassword: string, newPassword: string) =>
    gql<{ changePassword: boolean }>(
      `mutation($o:String!,$n:String!){changePassword(oldPassword:$o,newPassword:$n)}`,
      { o: oldPassword, n: newPassword },
    ).then((d) => d.changePassword),

  // users
  users: () => gql<{ users: User[] }>(`{users{${USER_FIELDS}}}`).then((d) => d.users),
  createUser: (input: Record<string, unknown>) =>
    gql<{ createUser: User }>(`mutation($i:CreateUserInput!){createUser(input:$i){${USER_FIELDS}}}`, { i: input }).then((d) => d.createUser),
  updateUser: (id: string, input: Record<string, unknown>) =>
    gql<{ updateUser: User }>(`mutation($id:ID!,$i:UpdateUserInput!){updateUser(id:$id,input:$i){${USER_FIELDS}}}`, { id, i: input }).then((d) => d.updateUser),
  deleteUser: (id: string) => gql<{ deleteUser: boolean }>(`mutation($id:ID!){deleteUser(id:$id)}`, { id }).then((d) => d.deleteUser),

  // teams
  teams: () => gql<{ teams: Team[] }>(`{teams{id name createdAt memberCount}}`).then((d) => d.teams),
  team: (id: string) => gql<{ team: Team | null }>(`query($id:ID!){team(id:$id){id name createdAt memberCount members{userId email name role addedAt}}}`, { id }).then((d) => d.team),
  createTeam: (name: string) => gql<{ createTeam: Team }>(`mutation($n:String!){createTeam(name:$n){id name createdAt memberCount}}`, { n: name }).then((d) => d.createTeam),
  deleteTeam: (id: string) => gql<{ deleteTeam: boolean }>(`mutation($id:ID!){deleteTeam(id:$id)}`, { id }).then((d) => d.deleteTeam),
  addTeamMember: (teamId: string, userId: string, role: string) =>
    gql<{ addTeamMember: boolean }>(`mutation($t:ID!,$u:ID!,$r:String){addTeamMember(teamId:$t,userId:$u,role:$r)}`, { t: teamId, u: userId, r: role }).then((d) => d.addTeamMember),
  removeTeamMember: (teamId: string, userId: string) =>
    gql<{ removeTeamMember: boolean }>(`mutation($t:ID!,$u:ID!){removeTeamMember(teamId:$t,userId:$u)}`, { t: teamId, u: userId }).then((d) => d.removeTeamMember),

  // dashboard / analytics
  stats: () => gql<{ stats: Stats }>(`{stats}`).then((d) => d.stats),
  analytics: () => gql<{ analytics: Analytics }>(`{analytics}`).then((d) => d.analytics),

  // signals
  signals: (filter: Record<string, unknown>, limit: number, offset: number) =>
    gql<{ signals: Signal[] }>(`query($f:SignalFilter,$l:Int,$o:Int){signals(filter:$f,limit:$l,offset:$o){${SIGNAL_FIELDS}}}`, { f: filter, l: limit, o: offset }).then((d) => d.signals),
  signalCount: (filter: Record<string, unknown>) =>
    gql<{ signalCount: number }>(`query($f:SignalFilter){signalCount(filter:$f)}`, { f: filter }).then((d) => d.signalCount),
  signal: (id: string) => gql<{ signal: Signal | null }>(`query($id:ID!){signal(id:$id){${SIGNAL_FIELDS}}}`, { id }).then((d) => d.signal),
  attributeDictionary: () =>
    gql<{ attributeDictionary: AttributeDefinition[] }>(`{attributeDictionary{key label kind description values{code label}}}`).then((d) => d.attributeDictionary),
  // Lightweight feed for the live map: only what a marker needs.
  // Events within a rolling time window (since ISO timestamp), optionally scoped
  // to a country (server-side) so a selected country shows its full picture.
  liveSignals: (since?: string, country?: string | null, limit = 500) => {
    const f: Record<string, unknown> = {};
    if (since) f.since = since;
    if (country) f.country = country;
    return gql<{ signals: LiveSignal[] }>(
      `query($f:SignalFilter,$l:Int){signals(filter:$f,limit:$l){id title country region city severity eventType lastSeenAt sourceCount sentiment influence relevance}}`,
      { f, l: limit },
    ).then((d) => d.signals);
  },

  // sources
  sources: (filter: SourceFilter = {}, limit = 50, offset = 0) =>
    gql<{ sources: Source[] }>(`{sources${sourceArgs(filter, { limit, offset })}{${SOURCE_LIST_FIELDS}}}`).then((d) => d.sources),
  sourceCount: (filter: SourceFilter = {}) =>
    gql<{ sourceCount: number }>(`{sourceCount${sourceArgs(filter)}}`).then((d) => d.sourceCount),
  sourceCoverage: () =>
    gql<{ sourceCoverage: SourceCoverage }>(
      `{sourceCoverage{byRegion{key count} byScope{key count} byOrgType{key count} byValidation{key count} byIndustry{key count} byCountry{key count} bySourceType{key count} byLanguage{key count}}}`,
    ).then((d) => d.sourceCoverage),
  source: (id: string) =>
    gql<{ source: Source | null }>(`query($id:ID!){source(id:$id){${SOURCE_FIELDS} config validationLogs{${VALIDATION_LOG_FIELDS}}}}`, { id }).then((d) => d.source),
  revalidateSource: (id: string) =>
    gql<{ revalidateSource: Source }>(`mutation($id:ID!){revalidateSource(id:$id){${SOURCE_FIELDS} validationLogs{${VALIDATION_LOG_FIELDS}}}}`, { id }).then((d) => d.revalidateSource),
  createSource: (input: Record<string, unknown>) =>
    gql<{ createSource: Source }>(`mutation($i:CreateSourceInput!){createSource(input:$i){${SOURCE_FIELDS}}}`, { i: input }).then((d) => d.createSource),
  updateSource: (id: string, input: Record<string, unknown>) =>
    gql<{ updateSource: Source }>(`mutation($id:ID!,$i:UpdateSourceInput!){updateSource(id:$id,input:$i){${SOURCE_FIELDS}}}`, { id, i: input }).then((d) => d.updateSource),
  deleteSource: (id: string) => gql<{ deleteSource: boolean }>(`mutation($id:ID!){deleteSource(id:$id)}`, { id }).then((d) => d.deleteSource),
  setSourceEnabled: (id: string, enabled: boolean) =>
    gql<{ setSourceEnabled: Source }>(`mutation($id:ID!,$e:Boolean!){setSourceEnabled(id:$id,enabled:$e){${SOURCE_FIELDS}}}`, { id, e: enabled }).then((d) => d.setSourceEnabled),
  triggerFetch: (id: string) => gql<{ triggerFetch: boolean }>(`mutation($id:ID!){triggerFetch(id:$id)}`, { id }).then((d) => d.triggerFetch),

  // articles / raw items
  articles: (vars: Record<string, unknown>) =>
    gql<{ articles: Page<ArticleRow> }>(`query($sourceId:ID,$search:String,$limit:Int,$offset:Int){articles(sourceId:$sourceId,search:$search,limit:$limit,offset:$offset){items{id title canonicalUrl summary publishedAt fetchedAt sourceId sourceName signalCount} total}}`, vars).then((d) => d.articles),
  article: (id: string) =>
    gql<{ article: ArticleDetail | null }>(`query($id:ID!){article(id:$id){id title canonicalUrl summary publishedAt fetchedAt sourceId sourceName signalCount body author language country contentHash tokenSet signals{id title relationType similarityScore}}}`, { id }).then((d) => d.article),
  rawItems: (vars: Record<string, unknown>) =>
    gql<{ rawItems: Page<RawItemRow> }>(`query($sourceId:ID,$status:String,$limit:Int,$offset:Int){rawItems(sourceId:$sourceId,status:$status,limit:$limit,offset:$offset){items{id sourceId sourceName sourceGuid rawUrl rawTitle status publishedAt fetchedAt} total}}`, vars).then((d) => d.rawItems),
  rawItem: (id: string) =>
    gql<{ rawItem: RawItemDetail | null }>(`query($id:ID!){rawItem(id:$id){id sourceId sourceName sourceGuid rawUrl rawTitle status publishedAt fetchedAt rawContent contentHash rawPayload}}`, { id }).then((d) => d.rawItem),

  // deliveries
  deliveries: (vars: Record<string, unknown>) =>
    gql<{ deliveries: Page<DeliveryRow> }>(`query($status:String,$subscriptionId:ID,$limit:Int,$offset:Int){deliveries(status:$status,subscriptionId:$subscriptionId,limit:$limit,offset:$offset){items{id subscriptionId subscriptionName channel signalId signalTitle status attempts createdAt deliveredAt failedAt errorMessage} total}}`, vars).then((d) => d.deliveries),
  delivery: (id: string) =>
    gql<{ delivery: DeliveryDetail | null }>(`query($id:ID!){delivery(id:$id){id subscriptionId subscriptionName channel signalId signalTitle status attempts createdAt deliveredAt failedAt errorMessage payload}}`, { id }).then((d) => d.delivery),
  retryDelivery: (id: string) => gql<{ retryDelivery: boolean }>(`mutation($id:ID!){retryDelivery(id:$id)}`, { id }).then((d) => d.retryDelivery),

  // subscriptions / subscribers
  subscriptions: () => gql<{ subscriptions: Subscription[] }>(`{subscriptions{id name channel enabled filter config createdAt}}`).then((d) => d.subscriptions),
  subscription: (id: string) => gql<{ subscription: Subscription | null }>(`query($id:ID!){subscription(id:$id){id subscriberId name channel enabled filter config createdAt}}`, { id }).then((d) => d.subscription),
  createSubscription: (input: Record<string, unknown>) =>
    gql<{ createSubscription: Subscription }>(`mutation($i:CreateSubscriptionInput!){createSubscription(input:$i){id name channel enabled filter config createdAt}}`, { i: input }).then((d) => d.createSubscription),
  updateSubscription: (id: string, input: Record<string, unknown>) =>
    gql<{ updateSubscription: Subscription }>(`mutation($id:ID!,$i:UpdateSubscriptionInput!){updateSubscription(id:$id,input:$i){id name channel enabled filter config createdAt}}`, { id, i: input }).then((d) => d.updateSubscription),
  testSubscription: (id: string) =>
    gql<{ testSubscription: { ok: boolean; channel: string; message: string } }>(`mutation($id:ID!){testSubscription(id:$id){ok channel message}}`, { id }).then((d) => d.testSubscription),
  deleteSubscription: (id: string) => gql<{ deleteSubscription: boolean }>(`mutation($id:ID!){deleteSubscription(id:$id)}`, { id }).then((d) => d.deleteSubscription),
  subscribers: () => gql<{ subscribers: Subscriber[] }>(`{subscribers{id name status createdAt subscriptionCount}}`).then((d) => d.subscribers),
  createSubscriber: (name: string) => gql<{ createSubscriber: Subscriber }>(`mutation($n:String!){createSubscriber(name:$n){id name status createdAt subscriptionCount}}`, { n: name }).then((d) => d.createSubscriber),
  deleteSubscriber: (id: string) => gql<{ deleteSubscriber: boolean }>(`mutation($id:ID!){deleteSubscriber(id:$id)}`, { id }).then((d) => d.deleteSubscriber),

  // entities (signals:read)
  entities: (filter: EntityFilter = {}, limit = 100) =>
    gql<{ entities: Entity[] }>(
      `query($s:String,$t:String,$l:Int){entities(search:$s,type:$t,limit:$l){name type signalCount}}`,
      { s: filter.search ?? null, t: filter.type ?? null, l: limit },
    ).then((d) => d.entities),

  // reference data
  countries: () =>
    gql<{ countries: Country[] }>(`{countries{code name flag currency capital capitalLat capitalLng}}`).then((d) => d.countries),

  // taxonomy
  taxonomy: () => gql<{ taxonomy: TaxonomyNode[] }>(`{taxonomy}`).then((d) => d.taxonomy),
  taxonomyStats: () => gql<{ taxonomyStats: Bucket[] }>(`{taxonomyStats{code count}}`).then((d) => d.taxonomyStats),

  // jobs
  jobs: (vars: Record<string, unknown>) =>
    gql<{ jobs: Page<Job> }>(`query($queue:String,$state:String,$limit:Int,$offset:Int){jobs(queue:$queue,state:$state,limit:$limit,offset:$offset){items{id queue state retryCount retryLimit createdAt startedAt completedAt lastError} total}}`, vars).then((d) => d.jobs),
  jobCounts: () => gql<{ jobCounts: Bucket[] }>(`{jobCounts{key count}}`).then((d) => d.jobCounts),
  retryJob: (id: string) => gql<{ retryJob: boolean }>(`mutation($id:ID!){retryJob(id:$id)}`, { id }).then((d) => d.retryJob),

  // LLM keys (settings:manage)
  llmKeys: () =>
    gql<{ llmKeys: LLMKey[] }>(`{llmKeys{${LLM_KEY_FIELDS}}}`).then((d) => d.llmKeys),
  llmStatus: () =>
    gql<{ llmStatus: LLMStatus }>(`{llmStatus{provider enabled source model hasSystemKey activeLabel}}`).then((d) => d.llmStatus),
  llmModels: () => gql<{ llmModels: string[] }>(`{llmModels}`).then((d) => d.llmModels),
  createLLMKey: (input: { provider?: string; label: string; key: string; model?: string }) =>
    gql<{ createLLMKey: LLMKey }>(`mutation($i:CreateLLMKeyInput!){createLLMKey(input:$i){${LLM_KEY_FIELDS}}}`, { i: input }).then((d) => d.createLLMKey),
  setActiveLLMKey: (id: string) =>
    gql<{ setActiveLLMKey: LLMKey }>(`mutation($id:ID!){setActiveLLMKey(id:$id){${LLM_KEY_FIELDS}}}`, { id }).then((d) => d.setActiveLLMKey),
  testLLMKey: (id: string) =>
    gql<{ testLLMKey: LLMTestResult }>(`mutation($id:ID!){testLLMKey(id:$id){ok status error}}`, { id }).then((d) => d.testLLMKey),
  deleteLLMKey: (id: string) =>
    gql<{ deleteLLMKey: boolean }>(`mutation($id:ID!){deleteLLMKey(id:$id)}`, { id }).then((d) => d.deleteLLMKey),

  // email connectors (settings:manage)
  emailConnectors: () =>
    gql<{ emailConnectors: EmailConnector[] }>(`{emailConnectors{${CONNECTOR_FIELDS}}}`).then((d) => d.emailConnectors),
  emailProviders: () =>
    gql<{ emailProviders: EmailProvider[] }>(`{emailProviders{code label host port security usernameHint secretHint help docsAnchor editable}}`).then((d) => d.emailProviders),
  createEmailConnector: (input: Record<string, unknown>) =>
    gql<{ createEmailConnector: EmailConnector }>(`mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){${CONNECTOR_FIELDS}}}`, { i: input }).then((d) => d.createEmailConnector),
  updateEmailConnector: (id: string, input: Record<string, unknown>) =>
    gql<{ updateEmailConnector: EmailConnector }>(`mutation($id:ID!,$i:UpdateEmailConnectorInput!){updateEmailConnector(id:$id,input:$i){${CONNECTOR_FIELDS}}}`, { id, i: input }).then((d) => d.updateEmailConnector),
  setActiveEmailConnector: (id: string) =>
    gql<{ setActiveEmailConnector: EmailConnector }>(`mutation($id:ID!){setActiveEmailConnector(id:$id){${CONNECTOR_FIELDS}}}`, { id }).then((d) => d.setActiveEmailConnector),
  testEmailConnector: (id: string) =>
    gql<{ testEmailConnector: ConnectorTestResult }>(`mutation($id:ID!){testEmailConnector(id:$id){ok status error}}`, { id }).then((d) => d.testEmailConnector),
  sendTestEmail: (id: string, to: string) =>
    gql<{ sendTestEmail: ConnectorTestResult }>(`mutation($id:ID!,$to:String!){sendTestEmail(id:$id,to:$to){ok error}}`, { id, to }).then((d) => d.sendTestEmail),
  deleteEmailConnector: (id: string) =>
    gql<{ deleteEmailConnector: boolean }>(`mutation($id:ID!){deleteEmailConnector(id:$id)}`, { id }).then((d) => d.deleteEmailConnector),

  // API keys — public REST API credentials (settings:manage)
  apiKeys: () => gql<{ apiKeys: ApiKey[] }>(`{apiKeys{${API_KEY_FIELDS}}}`).then((d) => d.apiKeys),
  apiScopes: () => gql<{ apiScopes: string[] }>(`{apiScopes}`).then((d) => d.apiScopes),
  createApiKey: (input: { name: string; scopes: string[]; rateLimitPerMin?: number; expiresAt?: string }) =>
    gql<{ createApiKey: ApiKey }>(`mutation($i:CreateApiKeyInput!){createApiKey(input:$i){${API_KEY_FIELDS} key}}`, { i: input }).then((d) => d.createApiKey),
  setApiKeyEnabled: (id: string, enabled: boolean) =>
    gql<{ setApiKeyEnabled: ApiKey }>(`mutation($id:ID!,$e:Boolean!){setApiKeyEnabled(id:$id,enabled:$e){${API_KEY_FIELDS}}}`, { id, e: enabled }).then((d) => d.setApiKeyEnabled),
  deleteApiKey: (id: string) =>
    gql<{ deleteApiKey: boolean }>(`mutation($id:ID!){deleteApiKey(id:$id)}`, { id }).then((d) => d.deleteApiKey),

  // accounts — multi-tenant SaaS tenants (accounts:manage)
  accounts: () => gql<{ accounts: Account[] }>(`{accounts{${ACCOUNT_FIELDS}}}`).then((d) => d.accounts),
  account: (id: string) =>
    gql<{ account: Account | null }>(`query($id:ID!){account(id:$id){${ACCOUNT_FIELDS}}}`, { id }).then((d) => d.account),
  createAccount: (input: { name: string; slug?: string; plan?: string }) =>
    gql<{ createAccount: Account }>(`mutation($i:CreateAccountInput!){createAccount(input:$i){${ACCOUNT_FIELDS}}}`, { i: input }).then((d) => d.createAccount),
  updateAccount: (id: string, input: { name?: string; status?: string; plan?: string }) =>
    gql<{ updateAccount: Account }>(`mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){${ACCOUNT_FIELDS}}}`, { id, i: input }).then((d) => d.updateAccount),

  // audit log (settings:manage)
  auditLogs: (filter: AuditFilter = {}, limit = 50, offset = 0) => {
    const args = sourceArgs(filter as Record<string, string>, { limit, offset });
    return gql<{ auditLogs: Page<AuditLog> }>(`{auditLogs${args}{items{id actorEmail actorRole action targetType targetId metadata createdAt} total}}`).then((d) => d.auditLogs);
  },

  // smart-signals relevance feed (For You)
  subscriptionFeed: (id: string, minScore = 0, limit = 40) =>
    gql<{ subscriptionFeed: FeedItem[] }>(
      `query($id:ID!,$m:Float,$l:Int){subscriptionFeed(id:$id,minScore:$m,limit:$l){id title summary eventType country region sentiment influence severity ageHours score reasons}}`,
      { id, m: minScore, l: limit },
    ).then((d) => d.subscriptionFeed),
  subscriptionInterests: (id: string) =>
    gql<{ subscriptionInterests: Record<string, number> }>(`query($id:ID!){subscriptionInterests(id:$id)}`, { id }).then((d) => d.subscriptionInterests),
  setSubscriptionInterests: (id: string, interests: Record<string, number>) =>
    gql<{ setSubscriptionInterests: { ok: boolean } }>(
      `mutation($id:ID!,$i:JSON){setSubscriptionInterests(id:$id,interests:$i){ok}}`,
      { id, i: interests },
    ).then((d) => d.setSubscriptionInterests),
  recordSignalFeedback: (subscriptionId: string, signalId: string, action: FeedbackAction) =>
    gql<{ recordSignalFeedback: boolean }>(
      `mutation($s:ID!,$g:ID!,$a:String!){recordSignalFeedback(subscriptionId:$s,signalId:$g,action:$a)}`,
      { s: subscriptionId, g: signalId, a: action },
    ).then((d) => d.recordSignalFeedback),
  draftProfileFromDocument: (text: string) =>
    gql<{ draftProfileFromDocument: ProfileDraft }>(
      `mutation($t:String!){draftProfileFromDocument(text:$t){name summary minScore minSeverity source interests reasons{key why origin}}}`,
      { t: text },
    ).then((d) => d.draftProfileFromDocument),
};

const LLM_KEY_FIELDS = `id provider label keyLast4 model isActive status lastTestedAt lastError createdBy createdAt updatedAt`;
const CONNECTOR_FIELDS = `id name provider host port security username secretLast4 fromEmail fromName isActive enabled status lastTestedAt lastError createdAt updatedAt`;
const API_KEY_FIELDS = `id name keyPrefix scopes rateLimitPerMin enabled expiresAt lastUsedAt requestCount createdBy createdAt`;
const ACCOUNT_FIELDS = `id name slug status plan createdAt updatedAt`;
