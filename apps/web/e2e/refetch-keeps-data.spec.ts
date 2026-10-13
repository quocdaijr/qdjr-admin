import { test, expect, type Page } from '@playwright/test';

// A failed background refetch (here: the reload a mutation triggers) must keep
// what is already on screen; only a first load with nothing cached shows the
// error. The reload is faked to fail with page.route.

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
}

const down = { status: 503, contentType: 'application/json', body: JSON.stringify({ data: null, error: { code: 'UNAVAILABLE', message: 'api down' } }) };

async function createTag(page: Page, name: string) {
  await page.getByRole('button', { name: 'New tag' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('textbox').first().fill(name);
  await dialog.getByRole('textbox').first().blur();
  await dialog.getByRole('button', { name: /^(Create|Save)$/ }).click();
  await expect(dialog).toBeHidden();
}

test('tags list keeps its rows when the reload after a change fails', async ({ page }) => {
  const suffix = Math.random().toString(36).slice(2, 8);
  await login(page);
  await page.goto('/admin/tags');
  await createTag(page, `e2e-keep-a-${suffix}`);
  await expect(page.getByRole('row', { name: new RegExp(`e2e-keep-a-${suffix}`) })).toBeVisible();

  await page.route('**/v1/admin/tags?**', (route) => (route.request().method() === 'GET' ? route.fulfill(down) : route.fallback()));
  await createTag(page, `e2e-keep-b-${suffix}`);
  await page.waitForTimeout(1500);
  await expect(page.getByRole('row', { name: new RegExp(`e2e-keep-a-${suffix}`) })).toBeVisible();

  await page.unroute('**/v1/admin/tags?**');
  await page.reload();
  for (const name of [`e2e-keep-a-${suffix}`, `e2e-keep-b-${suffix}`]) {
    const row = page.getByRole('row', { name: new RegExp(name) });
    await row.getByRole('button', { name: 'Actions' }).click();
    await page.getByRole('menuitem', { name: 'Delete' }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
    await expect(page.getByRole('row', { name: new RegExp(name) })).toHaveCount(0);
  }
});

test('settings form stays on screen when the reload after saving fails', async ({ page }) => {
  await login(page);
  await page.goto('/admin/settings');
  const save = page.getByRole('button', { name: 'Save', exact: true });
  await expect(save).toBeVisible();
  await page.route('**/v1/admin/site-settings', (route) => (route.request().method() === 'GET' ? route.fulfill(down) : route.fallback()));
  await save.click();
  await page.waitForTimeout(1500);
  await expect(save).toBeVisible();
  await expect(page.getByText('Contact email')).toBeVisible();
});
