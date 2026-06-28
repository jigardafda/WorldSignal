import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    include: ["src/**/*.test.{ts,tsx}"], // unit tests only; e2e/ runs under Playwright
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      // main.tsx is the DOM bootstrap entrypoint (not unit-testable); exclude
      // tests and test helpers from the coverage denominator.
      exclude: ["src/main.tsx", "src/**/*.test.{ts,tsx}", "src/test/**", "src/vite-env.d.ts"],
      // "95% coverage" = the headline statements/lines metric (currently ~99%).
      // Functions/branches held to strong, realistic floors (JSX render-props and
      // defensive `?? "—"` branches are costly to fully exercise).
      thresholds: { statements: 95, lines: 95, functions: 90, branches: 85 },
    },
  },
});
