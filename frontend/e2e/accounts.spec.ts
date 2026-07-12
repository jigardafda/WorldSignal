import { expect, test, type Page } from "@playwright/test";

const ADMIN_EMAIL = "admin@worldsignal.local";
const ADMIN_PASSWORD = "admin12345";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByTestId("email").fill(ADMIN_EMAIL);
  await page.getByTestId("password").fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", level: 2 })).toBeVisible();
}

test.describe("accounts admin (multi-tenant)", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("lists the default tenant and creates a new account", async ({ page }) => {
    await page.getByRole("link", { name: "Accounts" }).click();
    await expect(page.getByRole("heading", { name: "Accounts", level: 2 })).toBeVisible();
    // The default account is seeded by the migration on boot.
    await expect(page.getByText("Default Account")).toBeVisible();
    await expect(page.getByText("default", { exact: true })).toBeVisible();

    // Create a tenant; the slug is derived from the name.
    await page.getByRole("button", { name: "Add account" }).click();
    await page.getByTestId("account-name").fill("Acme Corp");
    await page.getByRole("button", { name: "Create" }).click();
    await expect(page.getByText("Acme Corp")).toBeVisible();
    await expect(page.getByText("acme-corp")).toBeVisible();

    // Suspend it, then reactivate.
    const row = page.getByRole("row", { name: /Acme Corp/ });
    await row.getByRole("button", { name: "Suspend" }).click();
    await expect(row.getByRole("button", { name: "Activate" })).toBeVisible();
  });
});
