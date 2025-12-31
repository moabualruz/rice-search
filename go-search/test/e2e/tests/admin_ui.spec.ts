import { test, expect } from "@playwright/test";

test.describe("Admin UI & Premium Theme", () => {
  test("Admin Dashboard loads correctly", async ({ page }) => {
    // Navigate to admin page
    await page.goto("/admin");

    // Should NOT redirect to /stores
    await expect(page).toHaveURL(/\/admin/);

    // Check for dashboard elements
    await expect(page.locator("h1")).toContainText("System Administration");
    await expect(page.locator("text=ML Models")).toBeVisible();
    await expect(page.locator("text=Stores & Indexes")).toBeVisible();
  });

  test("Theme toggle works and persists", async ({ page }) => {
    await page.goto("/");

    // Check initial state (should be system or previously set, but we can reset)
    // We'll just cycle and check class changes on html element
    const html = page.locator("html");
    const toggle = page.locator("#theme-toggle-btn");

    // Click toggle
    await toggle.click();

    // Check for class change. Since we don't know start state, we just verify it changes
    // or we can force a specific state via localStorage in a beforeEach if needed.
    // simpler: check if classList contains 'dark' after cycling enough times

    // Let's try to set a specific theme via script first to test stability
    await page.evaluate(() => localStorage.setItem("theme", "light"));
    await page.reload();
    await expect(html).not.toHaveClass(/dark/);

    // Click to go to Dark
    await toggle.click();
    await expect(html).toHaveClass(/dark/);

    // Click to go to System (cycle)
    await toggle.click();
    await expect(toggle).toBeVisible();
  });

  test("Sidebar collapses and expands", async ({ page }) => {
    await page.goto("/");

    const sidebar = page.locator("#sidebar");
    const toggle = page.locator("#sidebar-toggle-icon").locator(".."); // button parent

    // Initial state (expanded)
    await expect(sidebar).toHaveClass(/w-64/);
    await expect(sidebar).not.toHaveClass(/w-16/);

    // Click toggle
    await toggle.click();

    // Check collapsed state
    await expect(sidebar).toHaveClass(/w-16/);
    await expect(sidebar).not.toHaveClass(/w-64/);

    // Reload to check persistence
    await page.reload();
    await expect(sidebar).toHaveClass(/w-16/);
  });

  test("Premium CSS Variables are applied", async ({ page }) => {
    await page.goto("/admin");

    // Check if a glass element exists and has backdrop-filter computed style
    const card = page.locator(".glass-panel").first();
    await expect(card).toBeVisible();

    // Verify computed style (proxy for premium.css being loaded)
    const backdropFilter = await card.evaluate((el) => {
      return window.getComputedStyle(el).backdropFilter;
    });
    // Validating that it's not 'none' (might be 'blur(16px)')
    expect(backdropFilter).not.toBe("none");
  });
});
