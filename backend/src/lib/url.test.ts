import { test } from "node:test";
import assert from "node:assert/strict";
import { canonicalizeUrl } from "./url.js";

test("strips tracking params and www, lowercases host", () => {
  assert.equal(
    canonicalizeUrl("https://WWW.Example.com/Story?utm_source=twitter&id=5&fbclid=abc"),
    "https://example.com/Story?id=5",
  );
});

test("removes fragment and trailing slash", () => {
  assert.equal(canonicalizeUrl("https://example.com/path/#section"), "https://example.com/path");
});

test("keeps root slash", () => {
  assert.equal(canonicalizeUrl("https://example.com/"), "https://example.com/");
});

test("two urls differing only by tracking params canonicalize equal", () => {
  const a = canonicalizeUrl("https://news.com/a?utm_campaign=x");
  const b = canonicalizeUrl("https://news.com/a?gclid=y");
  assert.equal(a, b);
});

test("returns null for empty input", () => {
  assert.equal(canonicalizeUrl(""), null);
  assert.equal(canonicalizeUrl(null), null);
  assert.equal(canonicalizeUrl("   "), null); // whitespace-only trims to empty
});

test("returns the raw value when not a parseable URL", () => {
  assert.equal(canonicalizeUrl("not a real url"), "not a real url");
});

test("drops default ports but keeps non-default ports", () => {
  assert.equal(canonicalizeUrl("https://example.com:443/x"), "https://example.com/x");
  assert.equal(canonicalizeUrl("http://example.com:80/y"), "http://example.com/y");
  assert.equal(canonicalizeUrl("https://example.com:8443/z"), "https://example.com:8443/z");
});
