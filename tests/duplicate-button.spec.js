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

test('duplicate carries over tags from original', async ({ page, request }) => {
  // Seed a subscription with tags via the API so the test doesn't depend on
  // whatever rows happen to be in the dev DB
  const cats = await (await request.get('/api/categories')).json();
  expect(cats.length).toBeGreaterThan(0);

  const unique = `Dup-Tags-${Date.now()}`;
  const createResp = await request.post('/api/subscriptions', {
    form: {
      name: unique,
      cost: '7.99',
      schedule: 'Monthly',
      status: 'Active',
      original_currency: 'USD',
      category_id: String(cats[0].id),
      tags: 'work, autopay, copytest',
    },
  });
  expect(createResp.ok()).toBeTruthy();
  const created = await createResp.json();

  // Trigger duplicate
  const dupResp = await request.post(`/api/subscriptions/${created.id}/duplicate`);
  expect(dupResp.ok()).toBeTruthy();
  const dup = await dupResp.json();
  expect(dup.name).toBe(`${unique} (Copy)`);

  // Re-fetch the duplicate to get its tags (the duplicate response is created BEFORE
  // tags are attached, so we GET it fresh to see the attached associations).
  const dupFull = await (await request.get(`/api/subscriptions/${dup.id}`)).json();
  const tagNames = (dupFull.tags || []).map(t => t.name).sort();
  expect(tagNames).toEqual(['autopay', 'copytest', 'work']);
});
