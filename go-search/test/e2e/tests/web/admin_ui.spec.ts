import { test, expect } from "@playwright/test";

test.describe("Admin UI", () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    console.log("Navigating to settings...");
    await page.goto("/admin/settings");
    console.log("Navigated.");
  });

  test.skip("should allow navigating admin dropdown", async ({ page }) => {
    const adminBtn = page.getByRole("button", { name: "Admin" });
    console.log("Hovering Admin button...");
    await expect(adminBtn).toBeVisible();
    await adminBtn.hover();
    console.log("Hovered.");
    // Wait for animation
    await page.waitForTimeout(500);

    // Check items
    const modelsLink = page.getByRole("link", { name: "Models" });
    console.log("Checking visibility of Models link...");
    await expect(modelsLink).toBeVisible();
    console.log("Models link visible.");
    const settingsLink = page.getByRole("link", { name: "Settings" });
    await expect(settingsLink).toBeVisible();

    // Click Models
    console.log("Clicking Models...");
    await modelsLink.click();
    await expect(page).toHaveURL(/\/admin\/models/);
    await expect(page.locator("h1")).toContainText("Model Management");
  });

  test("should toggle checkboxes in settings", async ({ page }) => {
    // Find a toggle, e.g., "Enable Tracking"
    // Since we are in E2E, we might update state.
    // We will click it and verify aria-checked flips.

    const toggle = page.locator('button[role="switch"]').first();
    if ((await toggle.count()) > 0) {
      const initialState = await toggle.getAttribute("aria-checked");
      await toggle.click();
      const newState = await toggle.getAttribute("aria-checked");
      expect(newState).not.toBe(initialState);

      // Toggle back to leave state as is
      await toggle.click();
    }
  });
});
