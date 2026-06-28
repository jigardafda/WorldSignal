import { defineConfig, devices } from "@playwright/test";

const E2E_DB = "postgresql://jigardafda@localhost:5432/worldsignal_e2e?sslmode=disable";

// End-to-end tests run the real Go backend (api role, LLM disabled) against a
// seeded Postgres DB, with the Vite dev server proxying /graphql to it.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 30_000,
  globalSetup: "./e2e/global-setup.ts",
  use: { baseURL: "http://localhost:5173", trace: "on-first-retry" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      // Run on 4100 so it never collides with a dev backend on the default 4000.
      command: "/tmp/ws-go",
      env: {
        DATABASE_URL: E2E_DB,
        ROLE: "api",
        OPENAI_API_KEY: "",
        PORT: "4100",
        HOST: "127.0.0.1",
        WEBHOOK_SIGNING_SECRET: "e2e-secret",
      },
      url: "http://127.0.0.1:4100/health",
      reuseExistingServer: false,
      timeout: 30_000,
    },
    {
      command: "npm run dev",
      env: { WS_BACKEND: "http://127.0.0.1:4100" },
      url: "http://localhost:5173",
      reuseExistingServer: false,
      timeout: 30_000,
    },
  ],
});
