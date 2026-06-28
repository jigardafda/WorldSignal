import { expect, test } from "@playwright/test";

test.describe("WorldSignal console against the Go backend", () => {
  test("dashboard shows seeded stats and the latest signal", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
    // Seeded signal surfaces in "Latest signals".
    await expect(page.getByText("Major earthquake strikes region")).toBeVisible();
    // Stat cards render (labels unique to the dashboard).
    await expect(page.getByText("Articles ingested")).toBeVisible();
    await expect(page.getByText("Deliveries sent")).toBeVisible();
  });

  test("signal explorer opens detail with sources and tags", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Signal Explorer" }).click();
    await expect(page.getByRole("heading", { name: "Signal Explorer" })).toBeVisible();
    await page.getByText("Major earthquake strikes region").click();
    await expect(page.getByText("Why it matters")).toBeVisible();
    await expect(page.getByText("Thousands affected.")).toBeVisible();
    await expect(page.getByRole("link", { name: "BBC World" })).toBeVisible();
  });

  test("sources tab lists the seeded source", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Sources" }).click();
    await expect(page.getByRole("heading", { name: "Sources" })).toBeVisible();
    await expect(page.getByText("BBC World")).toBeVisible();
    await expect(page.getByText("90%")).toBeVisible();
  });

  test("taxonomy tab renders the closed vocabulary", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Taxonomy" }).click();
    await expect(page.getByRole("heading", { name: "WorldSignal Taxonomy" })).toBeVisible();
    await expect(page.getByText("DISASTER.EARTHQUAKE").first()).toBeVisible();
  });

  test("creating a source via the UI hits the Go GraphQL mutation", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Sources" }).click();
    await page.getByPlaceholder("Name").fill("E2E Feed");
    await page.getByPlaceholder("RSS/Atom URL").fill("https://e2e.example/feed");
    await page.getByRole("button", { name: "Add source" }).click();
    await expect(page.getByText(/Source added/)).toBeVisible();
    await expect(page.getByText("E2E Feed")).toBeVisible();
  });
});
