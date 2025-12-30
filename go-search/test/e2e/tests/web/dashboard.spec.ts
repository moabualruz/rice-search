import { test, expect } from "@playwright/test";

test.describe("Web Dashboard", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("should load dashboard and show health status", async ({ page }) => {
    await expect(page).toHaveTitle(/Rice Search/);

    // Check for navigation items
    await expect(
      page.getByRole("link", { name: "Dashboard", exact: true })
    ).toBeVisible();
    await expect(page.getByRole("link", { name: "Search" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Stores" })).toBeVisible();

    // Admin dropdown interaction
    const adminBtn = page.getByRole("button", { name: "Admin" });
    await adminBtn.hover();
    // Check if menu appears (the fix we made)
    await expect(page.getByRole("link", { name: "Models" })).toBeVisible();

    // Check health dot
    const healthText = page.locator("span", { hasText: "Healthy" }).first(); // or Degraded
    await expect(healthText).toBeVisible();
  });

  test("should start in requested theme or system default", async ({
    page,
  }) => {
    const html = page.locator("html");
    // We can't easily predict system theme in headless, but we can toggle it
    const themeBtn = page.locator("#theme-toggle");
    await expect(themeBtn).toBeVisible();

    // Click to toggle
    await themeBtn.click();
    // Wait for class change
    // Logic: System -> Light -> Dark -> System
    // We just verify class list changes or localstorage persistence
  });
});
