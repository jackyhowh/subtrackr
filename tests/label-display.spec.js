// @ts-check
const { test, expect } = require('@playwright/test');

test('label field renders under subscription name', async ({ page }) => {
  await page.goto('/subscriptions');
  await page.waitForLoadState('networkidle');
  await expect(page.getByText('example.com').first()).toBeVisible();
  await page.screenshot({ path: 'screenshots/v0.6.0-label.png', fullPage: true });
});

test('label input visible in edit form', async ({ page }) => {
  await page.goto('/subscriptions');
  await page.waitForLoadState('networkidle');
  await page.locator('button[title="Edit"]').first().click();
  await page.waitForSelector('input[name="label"]', { timeout: 5000 });
  await expect(page.locator('input[name="label"]')).toBeVisible();
  await page.screenshot({ path: 'screenshots/v0.6.0-label-form.png', fullPage: true });
});
