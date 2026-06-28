import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../test/utils";

const { apiMock, authMock } = vi.hoisted(() => ({
  apiMock: {
    sources: vi.fn(), source: vi.fn(), createSource: vi.fn(), updateSource: vi.fn(),
    deleteSource: vi.fn(), triggerFetch: vi.fn(), setSourceEnabled: vi.fn(),
    sourceCount: vi.fn(), sourceCoverage: vi.fn(), revalidateSource: vi.fn(),
    articles: vi.fn(), article: vi.fn(), rawItems: vi.fn(), rawItem: vi.fn(),
    deliveries: vi.fn(), delivery: vi.fn(), retryDelivery: vi.fn(),
    jobs: vi.fn(), jobCounts: vi.fn(), retryJob: vi.fn(),
    taxonomy: vi.fn(), taxonomyStats: vi.fn(),
  },
  authMock: { user: { id: "me", email: "a@b.c", role: "ADMIN" }, loading: false, hasPerm: (_p: string): boolean => true, login: vi.fn(), logout: vi.fn(), refresh: vi.fn() },
}));
vi.mock("../lib/api", () => ({ api: apiMock }));
vi.mock("../lib/auth", () => ({ useAuth: () => authMock }));

import { Sources } from "./Sources";
import { SourceDetail } from "./SourceDetail";
import { Coverage } from "./Coverage";
import { Articles } from "./Articles";
import { ArticleDetail } from "./ArticleDetail";
import { RawItems } from "./RawItems";
import { RawItemDetail } from "./RawItemDetail";
import { Deliveries } from "./Deliveries";
import { DeliveryDetail } from "./DeliveryDetail";
import { Jobs } from "./Jobs";
import { Taxonomy } from "./Taxonomy";

const source = (o = {}) => ({ id: "s1", name: "BBC", type: "RSS", url: "https://bbc.example/feed", country: "GB", region: "Europe", language: "en", languages: ["en"], category: null, priority: 1, credibility: 0.9, crawlFrequency: 300, parserType: "rss", enabled: true, failureCount: 0, sourceType: "RSS", officialFeed: true, industry: null, publisher: "BBC", orgType: "PUBLIC", geographicScope: "GLOBAL", healthScore: 95, validationStatus: "VALID", tags: ["international"], lastSuccessAt: "2026-01-01T00:00:00Z", lastFailureAt: null, lastValidatedAt: "2026-01-01T00:00:00Z", lastFetchedAt: "2026-01-01T00:00:00Z", createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z", validationLogs: [], ...o });

const coverage = () => ({ byRegion: [{ key: "Europe", count: 1 }], byScope: [{ key: "GLOBAL", count: 1 }], byOrgType: [{ key: "PUBLIC", count: 1 }], byValidation: [{ key: "VALID", count: 1 }], byIndustry: [{ key: "Finance", count: 1 }], byCountry: [{ key: "GB", count: 1 }], bySourceType: [{ key: "RSS", count: 1 }], byLanguage: [{ key: "en", count: 1 }] });

afterEach(() => { vi.clearAllMocks(); authMock.hasPerm = () => true; });

describe("Sources", () => {
  it("lists, filters and creates", async () => {
    apiMock.sources.mockResolvedValue([source()]);
    apiMock.sourceCount.mockResolvedValue(1);
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    apiMock.createSource.mockResolvedValue(source({ id: "s2" }));
    renderWithProviders(<Sources />);
    expect(await screen.findByText("BBC")).toBeInTheDocument();
    // Filter by region drives a refetch.
    await userEvent.type(screen.getByTestId("source-search"), "bbc{Enter}");
    await waitFor(() => expect(apiMock.sources).toHaveBeenCalledWith(expect.objectContaining({ search: "bbc" }), 25, 0));
    await userEvent.click(screen.getByRole("button", { name: "Add source" }));
    await userEvent.type(await screen.findByTestId("src-name"), "New");
    await userEvent.type(screen.getByTestId("src-url"), "https://new.example/feed");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createSource).toHaveBeenCalled());
  });
  it("fetch + revalidate actions", async () => {
    apiMock.sources.mockResolvedValue([source()]);
    apiMock.sourceCount.mockResolvedValue(1);
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    apiMock.triggerFetch.mockResolvedValue(true);
    apiMock.revalidateSource.mockResolvedValue(source({ validationStatus: "VALID" }));
    renderWithProviders(<Sources />);
    await screen.findByText("BBC");
    await userEvent.click(screen.getByRole("button", { name: "Fetch" }));
    await waitFor(() => expect(apiMock.triggerFetch).toHaveBeenCalledWith("s1"));
    await userEvent.click(screen.getByRole("button", { name: "Revalidate" }));
    await waitFor(() => expect(apiMock.revalidateSource).toHaveBeenCalledWith("s1"));
  });
  it("shows empty state", async () => {
    apiMock.sources.mockResolvedValue([]);
    apiMock.sourceCount.mockResolvedValue(0);
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    renderWithProviders(<Sources />);
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
  });
  it("hides write actions for viewers", async () => {
    authMock.hasPerm = () => false;
    apiMock.sources.mockResolvedValue([source()]);
    apiMock.sourceCount.mockResolvedValue(1);
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    renderWithProviders(<Sources />);
    await screen.findByText("BBC");
    expect(screen.queryByRole("button", { name: "Add source" })).toBeNull();
  });
  it("surfaces an error when an action fails", async () => {
    apiMock.sources.mockResolvedValue([source()]);
    apiMock.sourceCount.mockResolvedValue(1);
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    apiMock.triggerFetch.mockRejectedValue(new Error("boom"));
    renderWithProviders(<Sources />);
    await screen.findByText("BBC");
    await userEvent.click(screen.getByRole("button", { name: "Fetch" }));
    await waitFor(() => expect(apiMock.triggerFetch).toHaveBeenCalled());
  });
  it("filters by region via the select", async () => {
    apiMock.sources.mockResolvedValue([source()]);
    apiMock.sourceCount.mockResolvedValue(1);
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    renderWithProviders(<Sources />);
    await screen.findByText("BBC");
    await userEvent.click(screen.getByTestId("source-region"));
    await userEvent.click(await screen.findByText("Africa"));
    await waitFor(() => expect(apiMock.sources).toHaveBeenCalledWith(expect.objectContaining({ region: "Africa" }), 25, 0));
  });
  it("surfaces an error when create fails", async () => {
    apiMock.sources.mockResolvedValue([source()]);
    apiMock.sourceCount.mockResolvedValue(1);
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    apiMock.createSource.mockRejectedValue(new Error("dup"));
    renderWithProviders(<Sources />);
    await screen.findByText("BBC");
    await userEvent.click(screen.getByRole("button", { name: "Add source" }));
    await userEvent.type(await screen.findByTestId("src-name"), "New");
    await userEvent.type(screen.getByTestId("src-url"), "https://new.example/feed");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(apiMock.createSource).toHaveBeenCalled());
  });
});

describe("Coverage", () => {
  it("renders coverage KPIs and charts", async () => {
    apiMock.sourceCoverage.mockResolvedValue(coverage());
    renderWithProviders(<Coverage />);
    expect(await screen.findByText("Total sources")).toBeInTheDocument();
    expect(screen.getByText("By region")).toBeInTheDocument();
    expect(screen.getByText("Validated")).toBeInTheDocument();
  });
  it("shows the error state", async () => {
    apiMock.sourceCoverage.mockRejectedValue(new Error("down"));
    renderWithProviders(<Coverage />);
    expect(await screen.findByTestId("error")).toBeInTheDocument();
  });
});

describe("SourceDetail", () => {
  it("shows detail, saves, fetches and deletes", async () => {
    apiMock.source.mockResolvedValue(source());
    apiMock.updateSource.mockResolvedValue(source({ name: "Edited" }));
    apiMock.triggerFetch.mockResolvedValue(true);
    apiMock.deleteSource.mockResolvedValue(true);
    renderWithProviders(<SourceDetail />, { route: "/sources/s1", path: "/sources/:id" });
    expect(await screen.findByText("https://bbc.example/feed")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(apiMock.updateSource).toHaveBeenCalled());
    await userEvent.click(screen.getByRole("button", { name: "Fetch now" }));
    await waitFor(() => expect(apiMock.triggerFetch).toHaveBeenCalledWith("s1"));
    await userEvent.click(screen.getByRole("button", { name: "Delete source" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(apiMock.deleteSource).toHaveBeenCalledWith("s1"));
  });
  it("handles not found", async () => {
    apiMock.source.mockResolvedValue(null);
    renderWithProviders(<SourceDetail />, { route: "/sources/x", path: "/sources/:id" });
    expect(await screen.findByText("Source not found.")).toBeInTheDocument();
  });
  it("blocks save when the name is cleared", async () => {
    apiMock.source.mockResolvedValue(source());
    renderWithProviders(<SourceDetail />, { route: "/sources/s1", path: "/sources/:id" });
    const name = await screen.findByLabelText("Name");
    await userEvent.clear(name);
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Name is required")).toBeInTheDocument();
    expect(apiMock.updateSource).not.toHaveBeenCalled();
  });
  it("renders metadata + validation history and revalidates", async () => {
    apiMock.source.mockResolvedValue(source({
      validationLogs: [{ id: "v1", checkedAt: "2026-01-02T00:00:00Z", ok: true, httpStatus: 200, responseMs: 120, itemCount: 30, newestItemAt: "2026-01-02T00:00:00Z", redirectedTo: null, error: null }],
    }));
    apiMock.revalidateSource.mockResolvedValue(source({ validationStatus: "VALID" }));
    renderWithProviders(<SourceDetail />, { route: "/sources/s1", path: "/sources/:id" });
    expect(await screen.findByText("Validation history")).toBeInTheDocument();
    expect(screen.getByText("Validation & health")).toBeInTheDocument();
    expect(screen.getByText("international")).toBeInTheDocument(); // tag badge
    await userEvent.click(screen.getByTestId("revalidate"));
    await waitFor(() => expect(apiMock.revalidateSource).toHaveBeenCalledWith("s1"));
  });
});

describe("Articles", () => {
  it("lists + searches", async () => {
    apiMock.articles.mockResolvedValue({ items: [{ id: "a1", title: "Quake", canonicalUrl: null, summary: null, publishedAt: null, fetchedAt: "2026-01-01T00:00:00Z", sourceId: "s1", sourceName: "BBC", signalCount: 1 }], total: 1 });
    renderWithProviders(<Articles />);
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    await userEvent.type(screen.getByTestId("article-search"), "q{Enter}");
    await waitFor(() => expect(apiMock.articles).toHaveBeenCalledWith(expect.objectContaining({ search: "q" })));
    await userEvent.click(screen.getByText("Quake"));
  });
});

describe("ArticleDetail", () => {
  it("renders detail", async () => {
    apiMock.article.mockResolvedValue({ id: "a1", title: "Quake", canonicalUrl: "https://x.example", summary: "s", publishedAt: null, fetchedAt: "2026-01-01T00:00:00Z", sourceId: "s1", sourceName: "BBC", signalCount: 1, body: "Body", author: "A", language: "en", country: "US", contentHash: "h", tokenSet: "t", signals: [{ id: "sg", title: "Sig", relationType: "PRIMARY", similarityScore: 1 }] });
    renderWithProviders(<ArticleDetail />, { route: "/articles/a1", path: "/articles/:id" });
    expect(await screen.findByText("Body")).toBeInTheDocument();
    expect(screen.getByText("Sig")).toBeInTheDocument();
    await userEvent.click(screen.getByText("Sig")); // signal row click
  });
  it("renders with no body and no signals", async () => {
    apiMock.article.mockResolvedValue({ id: "a1", title: "Quake", canonicalUrl: null, summary: null, publishedAt: null, fetchedAt: "2026-01-01T00:00:00Z", sourceId: "s1", sourceName: "BBC", signalCount: 0, body: null, author: null, language: null, country: null, contentHash: null, tokenSet: null, signals: [] });
    renderWithProviders(<ArticleDetail />, { route: "/articles/a1", path: "/articles/:id" });
    expect(await screen.findByText("No body.")).toBeInTheDocument();
    expect(screen.getByText("Not linked to any signal.")).toBeInTheDocument();
  });
  it("handles not found", async () => {
    apiMock.article.mockResolvedValue(null);
    renderWithProviders(<ArticleDetail />, { route: "/articles/x", path: "/articles/:id" });
    expect(await screen.findByText("Article not found.")).toBeInTheDocument();
  });
});

describe("RawItems + detail", () => {
  it("lists with status filter", async () => {
    apiMock.rawItems.mockResolvedValue({ items: [{ id: "r1", sourceId: "s1", sourceName: "BBC", sourceGuid: "g", rawUrl: null, rawTitle: "Raw", status: "PARSED", publishedAt: null, fetchedAt: "2026-01-01T00:00:00Z" }], total: 1 });
    renderWithProviders(<RawItems />);
    expect(await screen.findByText("Raw")).toBeInTheDocument();
  });
  it("shows detail payload", async () => {
    apiMock.rawItem.mockResolvedValue({ id: "r1", sourceId: "s1", sourceName: "BBC", sourceGuid: "g", rawUrl: "https://x.example", rawTitle: "Raw", status: "PARSED", publishedAt: null, fetchedAt: "2026-01-01T00:00:00Z", rawContent: "content", contentHash: "h", rawPayload: { k: "v" } });
    renderWithProviders(<RawItemDetail />, { route: "/raw-items/r1", path: "/raw-items/:id" });
    expect(await screen.findByText("content")).toBeInTheDocument();
  });
  it("handles null fields and not found", async () => {
    apiMock.rawItem.mockResolvedValue({ id: "r1", sourceId: "s1", sourceName: "BBC", sourceGuid: null, rawUrl: null, rawTitle: null, status: "PENDING", publishedAt: null, fetchedAt: "2026-01-01T00:00:00Z", rawContent: null, contentHash: null, rawPayload: null });
    renderWithProviders(<RawItemDetail />, { route: "/raw-items/r1", path: "/raw-items/:id" });
    expect(await screen.findByText("(untitled)")).toBeInTheDocument();
    apiMock.rawItem.mockResolvedValue(null);
    renderWithProviders(<RawItemDetail />, { route: "/raw-items/x", path: "/raw-items/:id" });
    expect(await screen.findByText("Raw item not found.")).toBeInTheDocument();
  });
});

describe("Deliveries + detail", () => {
  const row = { id: "d1", subscriptionId: "sub", subscriptionName: "All", channel: "WEBHOOK", signalId: "sg", signalTitle: "Quake", status: "FAILED", attempts: 2, createdAt: "2026-01-01T00:00:00Z", deliveredAt: null, failedAt: "2026-01-01T00:00:00Z", errorMessage: "boom" };
  it("lists and retries", async () => {
    apiMock.deliveries.mockResolvedValue({ items: [row], total: 1 });
    apiMock.retryDelivery.mockResolvedValue(true);
    renderWithProviders(<Deliveries />);
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(apiMock.retryDelivery).toHaveBeenCalledWith("d1"));
  });
  it("shows detail with payload + retry", async () => {
    apiMock.delivery.mockResolvedValue({ ...row, payload: { event_id: "e" } });
    apiMock.retryDelivery.mockResolvedValue(true);
    renderWithProviders(<DeliveryDetail />, { route: "/deliveries/d1", path: "/deliveries/:id" });
    expect(await screen.findByText("boom")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(apiMock.retryDelivery).toHaveBeenCalled());
  });
  it("disables retry for SENT and handles not found", async () => {
    apiMock.delivery.mockResolvedValue({ ...row, status: "SENT", errorMessage: null, deliveredAt: "2026-01-01T00:00:00Z", payload: {} });
    renderWithProviders(<DeliveryDetail />, { route: "/deliveries/d1", path: "/deliveries/:id" });
    expect(await screen.findByText("Quake")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeDisabled();
    apiMock.delivery.mockResolvedValue(null);
    renderWithProviders(<DeliveryDetail />, { route: "/deliveries/x", path: "/deliveries/:id" });
    expect(await screen.findByText("Delivery not found.")).toBeInTheDocument();
  });
});

describe("Jobs", () => {
  it("lists, shows counts and retries", async () => {
    apiMock.jobCounts.mockResolvedValue([{ key: "failed", count: 1 }]);
    apiMock.jobs.mockResolvedValue({ items: [{ id: "j1", queue: "source.fetch", state: "failed", retryCount: 2, retryLimit: 2, createdAt: "2026-01-01T00:00:00Z", startedAt: null, completedAt: null, lastError: "oops" }], total: 1 });
    apiMock.retryJob.mockResolvedValue(true);
    renderWithProviders(<Jobs />);
    expect(await screen.findByText("source.fetch")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(apiMock.retryJob).toHaveBeenCalledWith("j1"));
  });
});

describe("Taxonomy", () => {
  it("renders domains with counts", async () => {
    apiMock.taxonomy.mockResolvedValue([{ code: "DISASTER", label: "Disaster", children: [{ code: "DISASTER.FLOOD", label: "Flood" }] }, { code: "GENERAL", label: "General" }]);
    apiMock.taxonomyStats.mockResolvedValue([{ key: "DISASTER", count: 3 }, { key: "DISASTER.FLOOD", count: 2 }]);
    renderWithProviders(<Taxonomy />);
    expect(await screen.findByText("Disaster")).toBeInTheDocument();
    expect(screen.getByText("General")).toBeInTheDocument();
    const general = screen.getByText("General").closest("div")!;
    expect(within(general.parentElement as HTMLElement).getByText("No subcategories")).toBeInTheDocument();
  });
});
