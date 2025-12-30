import { test, expect } from '@playwright/test';

test.describe('Admin UI', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/admin/settings');
    });

    test('should allow navigating admin dropdown', async ({ page }) => {
        const adminBtn = page.getByRole('button', { name: 'Admin' });
        // Hover
        await adminBtn.hover();
        
        // Check items
        const modelsLink = page.getByRole('link', { name: 'Models' });
        await expect(modelsLink).toBeVisible();
        const settingsLink = page.getByRole('link', { name: 'Settings' });
        await expect(settingsLink).toBeVisible();

        // Click Models
        await modelsLink.click();
        await expect(page).toHaveURL(/\/admin\/models/);
        await expect(page.locator('h1')).toContainText('Model Management');
    });

    test('should toggle checkboxes in settings', async ({ page }) => {
        // Find a toggle, e.g., "Enable Tracking"
        // Since we are in E2E, we might update state. 
        // We will click it and verify aria-checked flips.
        
        const toggle = page.locator('button[role="switch"]').first();
        if (await toggle.count() > 0) {
             const initialState = await toggle.getAttribute('aria-checked');
             await toggle.click();
             const newState = await toggle.getAttribute('aria-checked');
             expect(newState).not.toBe(initialState);
             
             // Toggle back to leave state as is
             await toggle.click();
        }
    });
});
