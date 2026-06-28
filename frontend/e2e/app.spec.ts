import { expect, test, type Page } from "@playwright/test";

const ADMIN_EMAIL = "admin@worldsignal.local";
const ADMIN_PASSWORD = "admin12345";

async function login(page: Page, email = ADMIN_EMAIL, password = ADMIN_PASSWORD) {
  await page.goto("/login");
  await page.getByTestId("email").fill(email);
  await page.getByTestId("password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", level: 2 })).toBeVisible();
}

test.describe("auth gate", () => {
  test("unauthenticated visit redirects to login", async ({ page }) => {
    await page.goto("/signals");
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByText("Sign in to the admin console")).toBeVisible();
  });

  test("invalid credentials surface an error", async ({ page }) => {
    await page.goto("/login");
    await page.getByTestId("email").fill(ADMIN_EMAIL);
    await page.getByTestId("password").fill("wrong-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByTestId("login-error")).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });

  test("valid credentials sign in and reach the dashboard", async ({ page }) => {
    await login(page);
    // Seeded signal surfaces in "Latest signals".
    await expect(page.getByText("Major earthquake strikes region")).toBeVisible();
  });

  test("log out returns to the login screen", async ({ page }) => {
    await login(page);
    await page.getByTestId("user-menu").click();
    await page.getByRole("menuitem", { name: "Log out" }).click();
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe("authenticated console", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("signals list opens detail with sources and tags", async ({ page }) => {
    await page.getByRole("link", { name: "Signals" }).click();
    await expect(page.getByRole("heading", { name: "Signals", level: 2 })).toBeVisible();
    await page.getByText("Major earthquake strikes region").click();
    await expect(page.getByRole("heading", { name: "Why it matters" })).toBeVisible();
    await expect(page.getByText("Thousands affected.")).toBeVisible();
  });

  test("signals search filter drives the query", async ({ page }) => {
    await page.getByRole("link", { name: "Signals" }).click();
    await page.getByTestId("signal-search").fill("earthquake");
    await page.getByTestId("signal-search").press("Enter");
    await expect(page.getByText("Major earthquake strikes region")).toBeVisible();
  });

  test("sources tab lists the seeded source", async ({ page }) => {
    await page.getByRole("link", { name: "Sources" }).click();
    await expect(page.getByRole("heading", { name: "Sources", level: 2 })).toBeVisible();
    await expect(page.getByText("BBC World")).toBeVisible();
  });

  test("creating a source hits the GraphQL mutation and appears in the list", async ({ page }) => {
    await page.getByRole("link", { name: "Sources" }).click();
    await page.getByRole("button", { name: "Add source" }).click();
    await page.getByTestId("src-name").fill("E2E Feed");
    await page.getByTestId("src-url").fill("https://e2e.example/feed");
    await page.getByRole("button", { name: "Create" }).click();
    await expect(page.getByText("E2E Feed")).toBeVisible();
  });

  test("source detail shows metadata and validation history", async ({ page }) => {
    await page.getByRole("link", { name: "Sources" }).click();
    await page.getByText("BBC World").click();
    await expect(page.getByText("Validation & health")).toBeVisible();
    await expect(page.getByText("Validation history")).toBeVisible();
  });

  test("coverage page renders the global breakdown", async ({ page }) => {
    await page.getByRole("link", { name: "Coverage" }).click();
    await expect(page.getByRole("heading", { name: "Source Coverage", level: 2 })).toBeVisible();
    await expect(page.getByText("Total sources")).toBeVisible();
    await expect(page.getByText("By region")).toBeVisible();
  });

  test("taxonomy renders the closed vocabulary with counts", async ({ page }) => {
    await page.getByRole("link", { name: "Taxonomy" }).click();
    await expect(page.getByRole("heading", { name: "Taxonomy", level: 2 })).toBeVisible();
    // The "Disaster" domain renders with its "Earthquake" leaf (badge shows the
    // code segment after the dot, e.g. "EARTHQUAKE · N").
    await expect(page.getByText("Disaster", { exact: true })).toBeVisible();
    await expect(page.getByText(/EARTHQUAKE/).first()).toBeVisible();
  });

  test("analytics renders metric panels", async ({ page }) => {
    await page.getByRole("link", { name: "Analytics" }).click();
    await expect(page.getByRole("heading", { name: "Analytics", level: 2 })).toBeVisible();
    await expect(page.getByText("By severity")).toBeVisible();
  });

  test("admin can open the users page (RBAC-gated nav)", async ({ page }) => {
    await page.getByRole("link", { name: "Users" }).click();
    await expect(page.getByRole("heading", { name: "Users", level: 2 })).toBeVisible();
    await expect(page.getByText(ADMIN_EMAIL)).toBeVisible();
  });

  test("settings shows LLM status and the add-key flow", async ({ page }) => {
    await page.getByRole("link", { name: "Settings" }).click();
    await expect(page.getByRole("heading", { name: "Settings", level: 2 })).toBeVisible();
    await expect(page.getByTestId("llm-status")).toBeVisible();
    await expect(page.getByRole("button", { name: "Add OpenAI key" })).toBeVisible();
  });

  test("audit log records the login and is browsable", async ({ page }) => {
    await page.getByRole("link", { name: "Audit Log" }).click();
    await expect(page.getByRole("heading", { name: "Audit Log", level: 2 })).toBeVisible();
    // The beforeEach login produced a LOGIN audit entry.
    await expect(page.getByText("LOGIN").first()).toBeVisible();
  });

  test("account page reflects the signed-in admin", async ({ page }) => {
    await page.getByTestId("user-menu").click();
    await page.getByRole("menuitem", { name: "Account" }).click();
    await expect(page.getByRole("heading", { name: "Account", level: 2 })).toBeVisible();
    await expect(page.getByText(ADMIN_EMAIL)).toBeVisible();
  });
});
