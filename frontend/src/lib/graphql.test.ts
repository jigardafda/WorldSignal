import { afterEach, describe, expect, it, vi } from "vitest";
import { getToken, gql, GqlError, setToken } from "./graphql";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

function stubFetch(impl: () => unknown) {
  vi.stubGlobal("fetch", vi.fn(impl as never));
}

describe("token storage", () => {
  it("sets, gets and clears the token", () => {
    expect(getToken()).toBeNull();
    setToken("abc");
    expect(getToken()).toBe("abc");
    setToken(null);
    expect(getToken()).toBeNull();
  });
});

describe("gql", () => {
  it("returns data and attaches the auth header when a token is set", async () => {
    setToken("tok");
    const fetchMock = vi.fn().mockResolvedValue({ json: async () => ({ data: { x: 1 } }) });
    vi.stubGlobal("fetch", fetchMock);
    const out = await gql<{ x: number }>("{x}");
    expect(out.x).toBe(1);
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe("Bearer tok");
  });

  it("omits auth header without a token", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ json: async () => ({ data: { ok: true } }) });
    vi.stubGlobal("fetch", fetchMock);
    await gql("{ok}");
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBeUndefined();
  });

  it("throws a GqlError on GraphQL errors", async () => {
    stubFetch(() => Promise.resolve({ json: async () => ({ errors: [{ message: "forbidden" }] }) }));
    await expect(gql("{x}")).rejects.toThrow("forbidden");
    try {
      await gql("{x}");
    } catch (e) {
      expect((e as GqlError).forbidden).toBe(true);
    }
  });

  it("flags unauthenticated errors", async () => {
    stubFetch(() => Promise.resolve({ json: async () => ({ errors: [{ message: "unauthenticated" }] }) }));
    try {
      await gql("{x}");
    } catch (e) {
      expect((e as GqlError).unauthenticated).toBe(true);
    }
  });

  it("throws on network failure", async () => {
    stubFetch(() => Promise.reject(new Error("down")));
    await expect(gql("{x}")).rejects.toThrow("Network error");
  });

  it("throws on invalid JSON", async () => {
    stubFetch(() => Promise.resolve({ json: async () => { throw new Error("bad"); } }));
    await expect(gql("{x}")).rejects.toThrow("Invalid server response");
  });

  it("throws on empty data", async () => {
    stubFetch(() => Promise.resolve({ json: async () => ({ data: null }) }));
    await expect(gql("{x}")).rejects.toThrow("Empty response");
  });
});
