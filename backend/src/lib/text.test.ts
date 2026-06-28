import { test } from "node:test";
import assert from "node:assert/strict";
import {
  contentHash,
  jaccard,
  tokenSetString,
  stripHtml,
  normalizeText,
  firstSentences,
} from "./text.js";

test("identical content yields identical hash", () => {
  assert.equal(contentHash("Big News", "Body here."), contentHash("Big News", "Body here."));
});

test("hash ignores case and punctuation", () => {
  assert.equal(contentHash("Big News!", "Body, here."), contentHash("big news", "body here"));
});

test("jaccard of identical token sets is 1", () => {
  const a = tokenSetString("Magnitude earthquake hits Mindanao region");
  assert.equal(jaccard(a, a), 1);
});

test("jaccard rewards overlap and ignores stopwords", () => {
  const a = tokenSetString("Earthquake hits the Mindanao region today");
  const b = tokenSetString("A strong earthquake struck Mindanao region");
  const score = jaccard(a, b);
  assert.ok(score > 0.3 && score < 1, `expected partial overlap, got ${score}`);
});

test("stripHtml removes tags and decodes entities", () => {
  assert.equal(stripHtml("<p>Hello &amp; <b>world</b></p>"), "Hello & world");
});

test("stripHtml drops script/style and handles empty input", () => {
  assert.equal(stripHtml("<style>x{}</style><script>1</script><p>hi</p>"), "hi");
  assert.equal(stripHtml(null), "");
  assert.equal(stripHtml(undefined), "");
});

test("normalizeText lowercases and strips punctuation", () => {
  assert.equal(normalizeText("Hello, WORLD!!  Foo-bar"), "hello world foo bar");
});

test("jaccard of disjoint sets is 0 and empty input is 0", () => {
  assert.equal(jaccard(tokenSetString("apple banana"), tokenSetString("zebra yak")), 0);
  assert.equal(jaccard("", "anything here"), 0);
});

test("firstSentences returns up to N sentences", () => {
  const out = firstSentences("First one. Second two. Third three.", 2);
  assert.equal(out, "First one. Second two.");
  // text with no sentence punctuation falls back to the whole string
  assert.equal(firstSentences("no punctuation here", 1), "no punctuation here");
});
