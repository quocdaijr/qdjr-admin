import { test, expect } from '@playwright/test';

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';

test('admin can log in and sees super_admin badge', async ({ page }) => {
  await page.goto('/login');

  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();

  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();

  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
  expect(page.url()).toContain('/admin');

  // The role badge appears next to the user menu button once /me resolves.
  await expect(page.getByText('super_admin', { exact: true }).first()).toBeVisible({
    timeout: 15_000,
  });
});
