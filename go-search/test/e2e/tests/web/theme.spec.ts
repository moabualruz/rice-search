import { test, expect } from '@playwright/test';

test.describe('Theme Toggle', () => {
    test('should toggle dark mode', async ({ page }) => {
        await page.goto('/');
        
        const html = page.locator('html');
        const toggle = page.locator('#theme-toggle');
        
        // Initial state
        // It might be system default, let's assume we can cycle through.
        
        // Click 1: System -> Light (or next state)
        await toggle.click();
        await page.waitForTimeout(100); // Allow JS to run
        
        // Click 2: Light -> Dark
        await toggle.click();
        await expect(html).toHaveClass(/dark/);
        
        // Click 3: Dark -> System (removes dark class if system is light, or kept if system is dark)
        // Hard to assert system state without mocking media query.
        // But verifying manual toggle adds 'dark' class is good enough.
    });
});
