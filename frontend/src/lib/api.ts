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
export interface Signal {
  id: string; title: string; summary: string;
  whatHappened?: string | null; whyItMatters?: string | null;
  status: string; severity: string; confidence: number;
  eventType?: string | null; country?: string | null; sourceCount: number;
  firstSeenAt: string; lastSeenAt: string;
  tags: SignalTag[]; sources: SignalSource[];
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
  tag?: string; enabled?: boolean;
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
export interface Subscriber { id: string; name: string; status: string; createdAt: string; subscriptionCount: number }
export interface Bucket { key: string; count: number }
export interface Analytics {
  signalsBySeverity: Bucket[]; signalsByStatus: Bucket[]; signalsByEventType: Bucket[];
  signalsByCountry: Bucket[]; signalsOverTime: Bucket[];
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

const SIGNAL_FIELDS = `id title summary whatHappened whyItMatters status severity confidence eventType country sourceCount firstSeenAt lastSeenAt tags{code confidence} sources{publisher url publishedAt}`;
const SOURCE_LIST_FIELDS = `id name type url country region language languages category priority credibility enabled failureCount sourceType officialFeed industry publisher orgType geographicScope healthScore validationStatus tags lastSuccessAt lastFailureAt lastValidatedAt`;
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
  deleteSubscription: (id: string) => gql<{ deleteSubscription: boolean }>(`mutation($id:ID!){deleteSubscription(id:$id)}`, { id }).then((d) => d.deleteSubscription),
  subscribers: () => gql<{ subscribers: Subscriber[] }>(`{subscribers{id name status createdAt subscriptionCount}}`).then((d) => d.subscribers),
  createSubscriber: (name: string) => gql<{ createSubscriber: Subscriber }>(`mutation($n:String!){createSubscriber(name:$n){id name status createdAt subscriptionCount}}`, { n: name }).then((d) => d.createSubscriber),
  deleteSubscriber: (id: string) => gql<{ deleteSubscriber: boolean }>(`mutation($id:ID!){deleteSubscriber(id:$id)}`, { id }).then((d) => d.deleteSubscriber),

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

  // audit log (settings:manage)
  auditLogs: (filter: AuditFilter = {}, limit = 50, offset = 0) => {
    const args = sourceArgs(filter as Record<string, string>, { limit, offset });
    return gql<{ auditLogs: Page<AuditLog> }>(`{auditLogs${args}{items{id actorEmail actorRole action targetType targetId metadata createdAt} total}}`).then((d) => d.auditLogs);
  },
};

const LLM_KEY_FIELDS = `id provider label keyLast4 model isActive status lastTestedAt lastError createdBy createdAt updatedAt`;
