import "../test-utils/openai-env.js"; // sets OPENAI_API_KEY before config/env evaluates
import { test, before, mock } from "node:test";
import assert from "node:assert/strict";

// Controls the mocked OpenAI client's behavior per test.
let mode = "ok";

function content(): string | null {
  switch (mode) {
    case "empty":
      return null;
    case "invalidtags":
      return JSON.stringify({
        title: "Has only invalid tags",
        summary: "s",
        tags: [{ code: "NOPE.DOES_NOT_EXIST", confidence: 0.9 }],
        severity: "LOW",
        confidence: 0.7,
      });
    case "notitle":
      return JSON.stringify({ summary: "missing required title" });
    case "emptytitle":
      return JSON.stringify({
        title: "",
        summary: "empty title falls back to input",
        tags: [{ code: "ECONOMY.MARKETS", confidence: 0.6 }],
      });
    default:
      return JSON.stringify({
        title: "Central bank raises rates",
        summary: "The bank hiked rates.",
        whatHappened: "Rate up 25bps.",
        whyItMatters: "Borrowing costs rise.",
        severity: "HIGH",
        confidence: 0.82,
        tags: [
          { code: "ECONOMY.INTEREST_RATES", confidence: 0.95 },
          { code: "TECHNOLOGY.AI", confidence: 0.2 },
        ],
      });
  }
}

class FakeOpenAI {
  chat = {
    completions: {
      create: async () => {
        if (mode === "throw") throw new Error("simulated API failure");
        return { choices: [{ message: { content: content() } }] };
      },
    },
  };
}

let gateway: typeof import("./gateway.js");
let enrich: typeof import("./enrich.js");

before(async () => {
  mock.module("openai", { defaultExport: FakeOpenAI });
  gateway = await import("./gateway.js");
  enrich = await import("./enrich.js");
});

test("jsonCompletion returns parsed object on success (and caches the client)", async () => {
  mode = "ok";
  const a = await gateway.jsonCompletion<{ title: string }>({ system: "s", user: "u" });
  assert.equal(a?.title, "Central bank raises rates");
  // second call exercises the cached-client branch
  const b = await gateway.jsonCompletion<{ title: string }>({ system: "s", user: "u" });
  assert.equal(b?.title, "Central bank raises rates");
});

test("jsonCompletion returns null on empty content", async () => {
  mode = "empty";
  assert.equal(await gateway.jsonCompletion({ system: "s", user: "u" }), null);
});

test("jsonCompletion returns null when the API throws", async () => {
  mode = "throw";
  assert.equal(await gateway.jsonCompletion({ system: "s", user: "u" }), null);
});

test("enrichArticle uses the LLM path and constrains tags to the taxonomy", async () => {
  mode = "ok";
  const r = await enrich.enrichArticle({ title: "x", body: "y", publisher: "Test" });
  assert.equal(r.source, "llm");
  assert.equal(r.severity, "HIGH");
  assert.ok(r.tags.every((t) => t.code.includes(".")));
  assert.ok(r.tags.some((t) => t.code === "ECONOMY.INTEREST_RATES"));
});

test("enrichArticle adds fallback tag when LLM returns only invalid codes", async () => {
  mode = "invalidtags";
  const r = await enrich.enrichArticle({ title: "x", body: "y" });
  assert.equal(r.source, "llm");
  assert.deepEqual(
    r.tags.map((t) => t.code),
    ["GENERAL.OTHER"],
  );
});

test("enrichArticle falls back to heuristic when LLM output fails validation", async () => {
  mode = "notitle";
  const r = await enrich.enrichArticle({
    title: "Earthquake strikes",
    body: "earthquake magnitude tremor",
  });
  assert.equal(r.source, "heuristic");
});

test("enrichArticle keeps the input title when the LLM returns an empty title", async () => {
  mode = "emptytitle";
  const r = await enrich.enrichArticle({ title: "Original Title", body: "markets shares nasdaq" });
  assert.equal(r.source, "llm");
  assert.equal(r.title, "Original Title");
});
