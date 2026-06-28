import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      // main.tsx is the DOM bootstrap entrypoint (not unit-testable); exclude
      // tests and test helpers from the coverage denominator.
      exclude: ["src/main.tsx", "src/**/*.test.{ts,tsx}", "src/test/**", "src/vite-env.d.ts"],
      thresholds: { statements: 95, lines: 95, functions: 95, branches: 90 },
    },
  },
});
