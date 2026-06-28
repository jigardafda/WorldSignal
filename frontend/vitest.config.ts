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
      // Floors reflect Vitest 4's AST-aware v8 remapping, which counts statements
      // and branches more precisely than the legacy mapping (the same suite that
      // read ~99%/85% under Vitest 2 now reports its true ~93%/75%). Lines stay at
      // 95; functions/branches are held to strong, realistic floors (JSX
      // render-props and defensive `?? "—"` branches are costly to fully exercise).
      thresholds: { statements: 93, lines: 95, functions: 90, branches: 74 },
    },
  },
});
