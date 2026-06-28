import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
export const E2E_DB = "postgresql://worldsignal:worldsignal@localhost:5432/worldsignal_e2e";

// globalSetup runs after Playwright starts the web servers but before tests; the
// Go backend reads the DB per-request, so seeding here is sufficient.
export default function globalSetup() {
  execFileSync("psql", [E2E_DB, "-v", "ON_ERROR_STOP=1", "-f", join(here, "seed.sql")], {
    stdio: "inherit",
  });
}
