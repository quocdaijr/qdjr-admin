import { test, expect, type Page } from '@playwright/test';

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';
const API_BASE = process.env.E2E_API_BASE ?? 'http://localhost:8080';

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
}

test('profile content JSON round-trips to the public API', async ({ page, request }) => {
  await login(page);
  await page.goto('/admin/profile');

  const editor = page.getByLabel('Site content (JSON)');
  await expect(editor).toBeVisible();
  const original = await editor.inputValue();
  const marker = `e2e-${Math.random().toString(36).slice(2, 8)}`;
  const next = JSON.parse(original || '{}');
  next.e2e = marker;

  await editor.fill(JSON.stringify(next, null, 2));
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Profile saved')).toBeVisible();

  const res = await request.get(`${API_BASE}/v1/profile`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.data.content.e2e).toBe(marker);

  // restore
  await editor.fill(original || '{}');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Profile saved')).toBeVisible();
});

test('invalid JSON is rejected before saving', async ({ page }) => {
  await login(page);
  await page.goto('/admin/profile');
  const editor = page.getByLabel('Site content (JSON)');
  await editor.fill('[1, 2]');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Must be a JSON object')).toBeVisible();
});
