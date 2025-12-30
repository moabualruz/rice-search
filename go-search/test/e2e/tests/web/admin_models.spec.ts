import { test, expect } from "@playwright/test";

test.describe("Admin Models", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/admin/models");
  });

  test("should show downloading state when download clicked", async ({
    page,
  }) => {
    // Find a model that is NOT downloaded
    // We look for a "Download" button.
    const downloadBtn = page.getByRole("button", { name: "Download" }).first();

    if ((await downloadBtn.count()) === 0) {
      console.log("No download buttons found, skipping download test");
      return;
    }

    // Get the model ID from the button's HX attributes or parent?
    // We just click it and expect the UI to change.
    await downloadBtn.click();

    // Expect "Downloading..." to appear in the card
    // The OOB swap replaces the button with "Downloading..." status
    await expect(page.getByText("Downloading...")).toBeVisible();

    // We can also check if the progress bar appears
    const progressBar = page.locator('[id^="progress-"]');
    await expect(progressBar).toBeVisible();
  });
});
