import { expect, test } from "@playwright/test";

test("the home page shows the backend's health status", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByText("Backend status: ok")).toBeVisible();
});

test("a route that doesn't exist shows the app's not-found view", async ({ page }) => {
  await page.goto("/no-such-page");

  await expect(page.getByRole("heading", { name: "Page not found" })).toBeVisible();
});
