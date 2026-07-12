import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./graphql", () => ({ gql: vi.fn() }));
import { gql } from "./graphql";
import { api } from "./api";

const mockGql = gql as unknown as ReturnType<typeof vi.fn>;

// One object carrying every field the wrappers unwrap.
const RESP: Record<string, unknown> = {
  login: { token: "t", user: { id: "u" } }, me: { id: "u" }, logout: true, changePassword: true,
  users: [{ id: "u" }], createUser: { id: "u" }, updateUser: { id: "u" }, deleteUser: true,
  teams: [{ id: "t" }], team: { id: "t" }, createTeam: { id: "t" }, deleteTeam: true,
  addTeamMember: true, removeTeamMember: true,
  stats: { sources: 1 }, analytics: { signalsBySeverity: [] },
  signals: [{ id: "s" }], signalCount: 3, signal: { id: "s" },
  sources: [{ id: "s" }], source: { id: "s" }, createSource: { id: "s" }, updateSource: { id: "s" },
  deleteSource: true, setSourceEnabled: { id: "s" }, triggerFetch: true,
  sourceCount: 7, sourceCoverage: { byRegion: [] }, revalidateSource: { id: "s" },
  llmKeys: [{ id: "k" }], llmStatus: { enabled: true }, llmModels: ["gpt-4o"], createLLMKey: { id: "k" },
  setActiveLLMKey: { id: "k" }, testLLMKey: { ok: true }, deleteLLMKey: true,
  articles: { items: [], total: 0 }, article: { id: "a" },
  rawItems: { items: [], total: 0 }, rawItem: { id: "r" },
  deliveries: { items: [], total: 0 }, delivery: { id: "d" }, retryDelivery: true,
  subscriptions: [{ id: "x" }], subscription: { id: "x" }, createSubscription: { id: "x" },
  updateSubscription: { id: "x" }, deleteSubscription: true, testSubscription: { ok: true, channel: "SSE", message: "sent" },
  subscriptionFeed: [{ id: "s", score: 9.2, reasons: ["tag:DISASTER"] }],
  subscriptionInterests: { "tag:DISASTER": 5 }, setSubscriptionInterests: { ok: true },
  recordSignalFeedback: true, draftProfileFromDocument: { name: "P", source: "llm", interests: {}, reasons: [] },
  subscribers: [{ id: "sb" }], createSubscriber: { id: "sb" }, deleteSubscriber: true,
  emailConnectors: [{ id: "c" }], emailProviders: [{ code: "GMAIL" }], createEmailConnector: { id: "c" },
  updateEmailConnector: { id: "c" }, setActiveEmailConnector: { id: "c" }, testEmailConnector: { ok: true },
  sendTestEmail: { ok: true }, deleteEmailConnector: true,
  apiKeys: [{ id: "k" }], apiScopes: ["signals:read"], createApiKey: { id: "k", key: "wsk_x" },
  setApiKeyEnabled: { id: "k" }, deleteApiKey: true,
  accounts: [{ id: "a" }], account: { id: "a" }, createAccount: { id: "a" }, updateAccount: { id: "a" },
  myAccount: { id: "a" }, myApiKeys: [{ id: "k" }], tenantApiScopes: ["signals:read"],
  createMyApiKey: { id: "k", key: "wsk_x" }, revokeMyApiKey: true,
  mySubscriptions: [{ id: "s" }], createMySubscription: { id: "s" }, updateMySubscription: { id: "s" }, deleteMySubscription: true,
  entities: [{ name: "Acme", type: "ORG", signalCount: 3 }],
  countries: [{ code: "US", name: "United States" }],
  taxonomy: [{ code: "C" }], taxonomyStats: [{ key: "C", count: 1 }],
  jobs: { items: [], total: 0 }, jobCounts: [{ key: "k", count: 1 }], retryJob: true,
  auditLogs: { items: [], total: 0 },
};

beforeEach(() => {
  mockGql.mockReset();
  mockGql.mockResolvedValue(RESP);
});

describe("api wrappers", () => {
  it("unwrap their respective fields and pass variables", async () => {
    expect((await api.login("e", "p"))).toEqual(RESP.login);
    expect(mockGql.mock.calls.at(-1)?.[1]).toEqual({ e: "e", p: "p" });
    expect(await api.me()).toEqual(RESP.me);
    expect(await api.logout()).toBe(true);
    expect(await api.changePassword("a", "b")).toBe(true);

    expect(await api.users()).toEqual(RESP.users);
    expect(await api.createUser({ email: "x" })).toEqual(RESP.createUser);
    expect(await api.updateUser("u", { role: "ADMIN" })).toEqual(RESP.updateUser);
    expect(await api.deleteUser("u")).toBe(true);

    expect(await api.teams()).toEqual(RESP.teams);
    expect(await api.team("t")).toEqual(RESP.team);
    expect(await api.createTeam("n")).toEqual(RESP.createTeam);
    expect(await api.deleteTeam("t")).toBe(true);
    expect(await api.addTeamMember("t", "u", "OWNER")).toBe(true);
    expect(await api.removeTeamMember("t", "u")).toBe(true);

    expect(await api.stats()).toEqual(RESP.stats);
    expect(await api.analytics()).toEqual(RESP.analytics);

    expect(await api.signals({}, 10, 0)).toEqual(RESP.signals);
    expect(await api.signalCount({})).toBe(3);
    expect(await api.signal("s")).toEqual(RESP.signal);

    expect(await api.sources()).toEqual(RESP.sources);
    expect(await api.source("s")).toEqual(RESP.source);
    expect(await api.createSource({ name: "n" })).toEqual(RESP.createSource);
    expect(await api.updateSource("s", { name: "n" })).toEqual(RESP.updateSource);
    expect(await api.deleteSource("s")).toBe(true);
    expect(await api.setSourceEnabled("s", false)).toEqual(RESP.setSourceEnabled);
    expect(await api.triggerFetch("s")).toBe(true);
    expect(await api.sources({ region: "Africa", enabled: true }, 25, 0)).toEqual(RESP.sources);
    expect(await api.sourceCount({ language: "en" })).toBe(7);
    expect(await api.sourceCoverage()).toEqual(RESP.sourceCoverage);
    expect(await api.revalidateSource("s")).toEqual(RESP.revalidateSource);

    expect(await api.llmKeys()).toEqual(RESP.llmKeys);
    expect(await api.llmStatus()).toEqual(RESP.llmStatus);
    expect(await api.llmModels()).toEqual(RESP.llmModels);
    expect(await api.createLLMKey({ label: "L", key: "sk-x" })).toEqual(RESP.createLLMKey);
    expect(await api.setActiveLLMKey("k")).toEqual(RESP.setActiveLLMKey);
    expect(await api.testLLMKey("k")).toEqual(RESP.testLLMKey);
    expect(await api.deleteLLMKey("k")).toBe(true);
    expect(await api.auditLogs({ search: "x" }, 50, 0)).toEqual(RESP.auditLogs);

    expect(await api.articles({})).toEqual(RESP.articles);
    expect(await api.article("a")).toEqual(RESP.article);
    expect(await api.rawItems({})).toEqual(RESP.rawItems);
    expect(await api.rawItem("r")).toEqual(RESP.rawItem);

    expect(await api.deliveries({})).toEqual(RESP.deliveries);
    expect(await api.delivery("d")).toEqual(RESP.delivery);
    expect(await api.retryDelivery("d")).toBe(true);

    expect(await api.subscriptions()).toEqual(RESP.subscriptions);
    expect(await api.subscription("x")).toEqual(RESP.subscription);
    expect(await api.createSubscription({ name: "n" })).toEqual(RESP.createSubscription);
    expect(await api.updateSubscription("x", { name: "n" })).toEqual(RESP.updateSubscription);
    expect(await api.deleteSubscription("x")).toBe(true);
    expect(await api.testSubscription("x")).toEqual(RESP.testSubscription);
    expect(mockGql.mock.calls.at(-1)?.[1]).toEqual({ id: "x" });

    // smart-signals / For You
    expect(await api.subscriptionFeed("x", 2, 40)).toEqual(RESP.subscriptionFeed);
    expect(mockGql.mock.calls.at(-1)?.[1]).toEqual({ id: "x", m: 2, l: 40 });
    expect(await api.subscriptionInterests("x")).toEqual(RESP.subscriptionInterests);
    expect(await api.setSubscriptionInterests("x", { "tag:DISASTER": 5 })).toEqual(RESP.setSubscriptionInterests);
    expect(await api.recordSignalFeedback("x", "s", "UP")).toBe(true);
    expect(await api.draftProfileFromDocument("a document long enough")).toEqual(RESP.draftProfileFromDocument);
    expect(await api.subscribers()).toEqual(RESP.subscribers);
    expect(await api.createSubscriber("n")).toEqual(RESP.createSubscriber);
    expect(await api.deleteSubscriber("sb")).toBe(true);

    expect(await api.emailConnectors()).toEqual(RESP.emailConnectors);
    expect(await api.emailProviders()).toEqual(RESP.emailProviders);
    expect(await api.createEmailConnector({ name: "n" })).toEqual(RESP.createEmailConnector);
    expect(await api.updateEmailConnector("c", { name: "n" })).toEqual(RESP.updateEmailConnector);
    expect(await api.setActiveEmailConnector("c")).toEqual(RESP.setActiveEmailConnector);
    expect(await api.testEmailConnector("c")).toEqual(RESP.testEmailConnector);
    expect(await api.sendTestEmail("c", "a@x.com")).toEqual(RESP.sendTestEmail);
    expect(await api.deleteEmailConnector("c")).toBe(true);

    expect(await api.apiKeys()).toEqual(RESP.apiKeys);
    expect(await api.apiScopes()).toEqual(RESP.apiScopes);
    expect(await api.createApiKey({ name: "n", scopes: ["signals:read"], rateLimitPerMin: 60 })).toEqual(RESP.createApiKey);
    expect(await api.setApiKeyEnabled("k", false)).toEqual(RESP.setApiKeyEnabled);
    expect(await api.deleteApiKey("k")).toBe(true);

    expect(await api.accounts()).toEqual(RESP.accounts);
    expect(await api.account("a")).toEqual(RESP.account);
    expect(await api.createAccount({ name: "Acme", slug: "acme", plan: "PRO" })).toEqual(RESP.createAccount);
    expect(mockGql.mock.calls.at(-1)?.[1]).toEqual({ i: { name: "Acme", slug: "acme", plan: "PRO" } });
    expect(await api.updateAccount("a", { status: "SUSPENDED" })).toEqual(RESP.updateAccount);

    expect(await api.myAccount()).toEqual(RESP.myAccount);
    expect(await api.myApiKeys()).toEqual(RESP.myApiKeys);
    expect(await api.tenantApiScopes()).toEqual(RESP.tenantApiScopes);
    expect(await api.createMyApiKey({ name: "k", scopes: ["signals:read"] })).toEqual(RESP.createMyApiKey);
    expect(await api.revokeMyApiKey("k")).toBe(true);
    expect(await api.mySubscriptions()).toEqual(RESP.mySubscriptions);
    expect(await api.createMySubscription({ name: "s" })).toEqual(RESP.createMySubscription);
    expect(await api.updateMySubscription("s", { enabled: false })).toEqual(RESP.updateMySubscription);
    expect(await api.deleteMySubscription("s")).toBe(true);

    expect(await api.entities({ search: "a", type: "ORG" }, 10)).toEqual(RESP.entities);
    expect(await api.entities()).toEqual(RESP.entities);
    expect(await api.countries()).toEqual(RESP.countries);
    expect(await api.taxonomy()).toEqual(RESP.taxonomy);
    expect(await api.taxonomyStats()).toEqual(RESP.taxonomyStats);

    expect(await api.jobs({})).toEqual(RESP.jobs);
    expect(await api.jobCounts()).toEqual(RESP.jobCounts);
    expect(await api.retryJob("j")).toBe(true);
  });
});
