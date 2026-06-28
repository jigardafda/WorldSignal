import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

function mockFetch(payload: unknown, ok = true) {
  const fn = vi.fn().mockResolvedValue({
    ok,
    json: async () => payload,
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("api", () => {
  it("stats unwraps data.stats", async () => {
    const fn = mockFetch({ data: { stats: { sources: 1, articles: 2, signals: 3, deliveriesSent: 4, deliveriesPending: 5 } } });
    const s = await api.stats();
    expect(s.sources).toBe(1);
    const [, init] = fn.mock.calls[0];
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body).query).toContain("stats");
  });

  it("signals passes filter + limit variables", async () => {
    const fn = mockFetch({ data: { signals: [{ id: "x" }] } });
    const out = await api.signals({ country: "US" }, 10);
    expect(out).toHaveLength(1);
    expect(JSON.parse(fn.mock.calls[0][1].body).variables).toEqual({ filter: { country: "US" }, limit: 10 });
  });

  it("signal fetches by id", async () => {
    const fn = mockFetch({ data: { signal: { id: "sig" } } });
    const out = await api.signal("sig");
    expect(out.id).toBe("sig");
    expect(JSON.parse(fn.mock.calls[0][1].body).variables).toEqual({ id: "sig" });
  });

  it("sources returns list", async () => {
    mockFetch({ data: { sources: [{ id: "s1" }, { id: "s2" }] } });
    expect(await api.sources()).toHaveLength(2);
  });

  it("createSource sends input", async () => {
    const fn = mockFetch({ data: { createSource: { id: "n", name: "N" } } });
    const out = await api.createSource({ name: "N", url: "u" });
    expect(out.name).toBe("N");
    expect(JSON.parse(fn.mock.calls[0][1].body).variables).toEqual({ input: { name: "N", url: "u" } });
  });

  it("setSourceEnabled sends id + enabled", async () => {
    const fn = mockFetch({ data: { setSourceEnabled: { id: "s1", enabled: false } } });
    const out = await api.setSourceEnabled("s1", false);
    expect(out.enabled).toBe(false);
    expect(JSON.parse(fn.mock.calls[0][1].body).variables).toEqual({ id: "s1", enabled: false });
  });

  it("fetchSource maps triggerFetch → queued", async () => {
    mockFetch({ data: { triggerFetch: true } });
    expect(await api.fetchSource("s1")).toEqual({ queued: true });
  });

  it("taxonomy returns nodes", async () => {
    mockFetch({ data: { taxonomy: [{ code: "POLITICS", label: "Politics" }] } });
    const out = await api.taxonomy();
    expect(out[0].code).toBe("POLITICS");
  });

  it("throws on GraphQL errors", async () => {
    mockFetch({ errors: [{ message: "boom" }] });
    await expect(api.stats()).rejects.toThrow("boom");
  });
});
