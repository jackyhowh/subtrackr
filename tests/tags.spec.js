// @ts-check
const { test, expect } = require('@playwright/test');

test('tag chips render under the subscription name', async ({ page }) => {
  await page.goto('/subscriptions');
  await page.waitForLoadState('networkidle');
  await expect(page.getByText('#work').first()).toBeVisible();
  await expect(page.getByText('#autopay').first()).toBeVisible();
  await expect(page.getByText('#important').first()).toBeVisible();
  await page.screenshot({ path: 'screenshots/v0.6.0-tags.png', fullPage: true });
});

test('tags input pre-fills in edit form', async ({ page }) => {
  // Open the form for subscription 35 directly (the one tagged with work/autopay/important)
  await page.goto('/form/subscription/35');
  await page.waitForSelector('input[name="tags"]', { timeout: 5000 });
  const value = await page.inputValue('input[name="tags"]');
  expect(value).toContain('work');
  expect(value).toContain('autopay');
});
