import { test } from "node:test";
import assert from "node:assert/strict";
import { logger } from "./logger.js";

test("logger exposes all levels and does not throw", () => {
  const log = logger("test");
  // exercise with and without the optional extra argument (both branches)
  assert.doesNotThrow(() => {
    log.info("info message");
    log.info("info with extra", { a: 1 });
    log.warn("warn message");
    log.warn("warn with extra", new Error("x"));
    log.error("error message");
    log.error("error with extra", { code: 500 });
    log.debug("debug message");
    log.debug("debug with extra", [1, 2]);
  });
});
