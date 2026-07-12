import { expect, test, type Page } from "@playwright/test";

async function login(page: Page, email: string, password: string) {
  await page.goto("/login");
  await page.getByTestId("email").fill(email);
  await page.getByTestId("password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", level: 2 })).toBeVisible();
}

test.describe("operator vs tenant consoles", () => {
  test("platform staff get the operator console", async ({ page }) => {
    await login(page, "admin@worldsignal.local", "admin12345");
    await expect(page.getByTestId("console-mode")).toHaveText("Operator");
    // Operator-only surfaces are present.
    await expect(page.getByRole("link", { name: "Sources" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Accounts" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Users" })).toBeVisible();
  });

  test("account users get the customer console with a tenant-only menu", async ({ page }) => {
    await login(page, "tenant@acme.test", "admin12345");
    await expect(page.getByTestId("console-mode")).toHaveText("Customer");

    // Tenant menu: shared-corpus reads + self-service.
    await expect(page.getByRole("link", { name: "Signals" })).toBeVisible();
    await expect(page.getByRole("link", { name: "API Keys" })).toBeVisible();
    await expect(page.getByRole("link", { name: "My Account" })).toBeVisible();

    // Operator surfaces are gone.
    await expect(page.getByRole("link", { name: "Sources" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Users" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Accounts" })).toHaveCount(0);

    // Self-service surface: the tenant reaches its own API-keys page (account-
    // scoped). The full create/reveal flow is covered by unit tests.
    await page.getByRole("link", { name: "API Keys" }).click();
    await expect(page.getByRole("heading", { name: "API Keys", level: 2 })).toBeVisible();
    await expect(page.getByText("scoped to your account")).toBeVisible();
    await expect(page.getByTestId("add-key")).toBeVisible();

    // And its own account page.
    await page.getByRole("link", { name: "My Account" }).click();
    await expect(page.getByText("tenant@acme.test")).toBeVisible();
  });
});
