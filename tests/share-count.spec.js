// @ts-check
const { test, expect } = require('@playwright/test');

test('shared subscription shows split badge with your share', async ({ page }) => {
  await page.goto('/subscriptions');
  await page.waitForLoadState('networkidle');
  await expect(page.getByText(/split 4 ways/).first()).toBeVisible();
  await expect(page.getByText(/your share \$4\.00/).first()).toBeVisible();
  await page.screenshot({ path: 'screenshots/v0.6.0-share.png', fullPage: true });
});

test('share_count field present in form, defaults to 1', async ({ page }) => {
  await page.goto('/form/subscription');
  await page.waitForSelector('input[name="share_count"]', { timeout: 5000 });
  const val = await page.inputValue('input[name="share_count"]');
  expect(val).toBe('1');
});
