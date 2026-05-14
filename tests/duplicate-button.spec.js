// @ts-check
const { test, expect } = require('@playwright/test');

test('duplicate button visible and creates a copy', async ({ page }) => {
  await page.goto('/subscriptions');
  await page.waitForLoadState('networkidle');

  const initialRows = await page.locator('tbody tr').count();
  expect(initialRows).toBeGreaterThan(0);

  page.on('dialog', async dialog => {
    expect(dialog.message()).toContain('Duplicate');
    await dialog.accept();
  });

  const firstDupBtn = page.locator('button[title="Duplicate"]').first();
  await expect(firstDupBtn).toBeVisible();
  await firstDupBtn.click();

  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);

  const newRows = await page.locator('tbody tr').count();
  expect(newRows).toBe(initialRows + 1);

  await expect(page.getByText(/\(Copy\)/).first()).toBeVisible();

  await page.screenshot({ path: 'screenshots/v0.6.0-duplicate.png', fullPage: true });
});
