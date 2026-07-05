import { describe, expect, it } from "vitest";
import { codeFor, LANGUAGES, type Channel, type Language } from "./codegen";

const opts = { baseUrl: "https://api.example.com", subscriptionId: "sub_123" };
const LANGS: Language[] = LANGUAGES.map((l) => l.id);

describe("codeFor", () => {
  it("returns null for EMAIL (delivered to recipients, no client code)", () => {
    for (const l of LANGS) expect(codeFor(l, "EMAIL", opts)).toBeNull();
  });

  const streaming: { ch: Channel; path: string }[] = [
    { ch: "SSE", path: "/v1/stream/sse?subscription=sub_123" },
    { ch: "POLLING", path: "/v1/stream/poll?subscription=sub_123" },
    { ch: "WEBSOCKET", path: "/v1/stream/ws?subscription=sub_123" },
  ];
  for (const { ch, path } of streaming) {
    it(`${ch}: every language embeds the endpoint + subscription id`, () => {
      for (const l of LANGS) {
        const code = codeFor(l, ch, opts)!;
        expect(code, `${l}/${ch}`).toContain(path);
        if (ch === "WEBSOCKET") expect(code, l).toContain("wss://api.example.com"); // http→ws
      }
    });
  }

  it("consumer snippets reference the API key", () => {
    for (const l of LANGS) {
      for (const ch of ["SSE", "POLLING", "WEBSOCKET"] as Channel[]) {
        expect(codeFor(l, ch, opts)!, `${l}/${ch}`).toContain("WORLDSIGNAL_API_KEY");
      }
    }
  });

  it("WEBHOOK snippets verify the signature (except the cURL note)", () => {
    for (const l of LANGS) {
      const code = codeFor(l, "WEBHOOK", opts)!;
      expect(code, l).toBeTruthy();
      if (l !== "curl") {
        expect(code, l).toContain("WORLDSIGNAL_WEBHOOK_SECRET");
        expect(code.toLowerCase(), l).toContain("signature");
      }
    }
  });

  it("TypeScript reuses the Node snippet", () => {
    for (const ch of ["SSE", "POLLING", "WEBSOCKET", "WEBHOOK"] as Channel[]) {
      expect(codeFor("typescript", ch, opts)).toBe(codeFor("node", ch, opts));
    }
  });

  it("falls back to placeholders for empty base URL / subscription id", () => {
    const code = codeFor("curl", "SSE", { baseUrl: "", subscriptionId: "" })!;
    expect(code).toContain("localhost:4000");
    expect(code).toContain("subscription=%3Csubscription-id%3E"); // encoded <subscription-id>
  });

  it("strips a trailing slash from the base URL", () => {
    const code = codeFor("curl", "POLLING", { baseUrl: "https://api.example.com/", subscriptionId: "s" })!;
    expect(code).toContain("https://api.example.com/v1/stream/poll");
    expect(code).not.toContain("com//v1");
  });
});
