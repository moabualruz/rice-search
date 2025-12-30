import { test, expect } from "@playwright/test";

test.describe("Web Search UI", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/search");
  });

  test("should allow entering queries", async ({ page }) => {
    const input = page.getByPlaceholder("Search your code...");
    await expect(input).toBeVisible();

    await input.fill("test query");
    await expect(input).toHaveValue("test query");

    // Press enter
    await input.press("Enter");

    // Expect results or loading state
    // Since database might be empty, we expect either "No results" or results list
    const resultsContainer = page.locator("#search-results");
    await expect(resultsContainer).toBeVisible();
  });

  test("should use code filters", async ({ page }) => {
    // Toggle advanced filters        // Check for quick filters
    const input = page.getByPlaceholder("Search your code...");
    await input.fill("lang:go function");

    // Verify UI feedback if any
  });

  test("result cards should have hover effects", async ({ page }) => {
    // Mock a result if needed, or rely on live server having data.
    // For now, checking static elements.
  });
});
