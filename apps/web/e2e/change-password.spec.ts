import { test, expect, type Page } from '@playwright/test';

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';
const TEMP_PASSWORD = 'Temporary-Pass-123';

async function login(page: Page, password: string) {
  await page.goto('/login');
  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
}

async function changePassword(page: Page, from: string, to: string) {
  await page.getByRole('button', { name: ADMIN_EMAIL }).click();
  await page.getByRole('menuitem', { name: 'Change password' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Current password').fill(from);
  await dialog.getByLabel('New password', { exact: true }).fill(to);
  await dialog.getByLabel('Confirm new password').fill(to);
  await dialog.getByRole('button', { name: 'Change password' }).click();
  await expect(page.getByText('Password changed')).toBeVisible();
}

test('change password, sign in with the new one, change it back', async ({ page }) => {
  await login(page, ADMIN_PASSWORD);

  // Wrong current password is reported on the field, not as a generic error.
  await page.getByRole('button', { name: ADMIN_EMAIL }).click();
  await page.getByRole('menuitem', { name: 'Change password' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Current password').fill('not-my-password');
  await dialog.getByLabel('New password', { exact: true }).fill(TEMP_PASSWORD);
  await dialog.getByLabel('Confirm new password').fill(TEMP_PASSWORD);
  await dialog.getByRole('button', { name: 'Change password' }).click();
  await expect(dialog.getByText('Current password is incorrect')).toBeVisible();
  await page.keyboard.press('Escape');

  await changePassword(page, ADMIN_PASSWORD, TEMP_PASSWORD);

  await page.getByRole('button', { name: ADMIN_EMAIL }).click();
  await page.getByRole('menuitem', { name: 'Sign out' }).click();
  await page.waitForURL(/\/login$/);

  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByText('email or password is incorrect')).toBeVisible();

  await login(page, TEMP_PASSWORD);
  await changePassword(page, TEMP_PASSWORD, ADMIN_PASSWORD);
});
