// @ts-check
const { test, expect } = require('@playwright/test');

test('imported Wallos subscriptions appear in list', async ({ page }) => {
  await page.goto('/subscriptions');
  await page.waitForLoadState('networkidle');
  await expect(page.getByText('Wallos Netflix').first()).toBeVisible();
  await expect(page.getByText('Wallos Annual').first()).toBeVisible();
});

test('Wallos import card visible on settings page', async ({ page }) => {
  await page.goto('/settings');
  await page.waitForLoadState('networkidle');
  await expect(page.getByRole('heading', { name: 'Import from Wallos' })).toBeVisible();
  await page.screenshot({ path: 'screenshots/v0.6.0-wallos-settings.png', fullPage: true });
});
