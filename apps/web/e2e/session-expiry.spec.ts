import { test, expect, type Page } from '@playwright/test';

// When the server keeps refusing the session and the client cannot recover,
// the admin must sign out and land on /login, never sit on a silent error.
// The API is faked with page.route, so no real session is touched.

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
}

const unauthenticatedRPC = { status: 401, contentType: 'application/json', body: JSON.stringify({ code: 'unauthenticated', message: 'invalid or expired token' }) };
const unauthenticatedREST = { status: 401, contentType: 'application/json', body: JSON.stringify({ data: null, error: { code: 'UNAUTHENTICATED', message: 'invalid or expired token' } }) };

test('Connect call still refused after a successful refresh signs out to /login', async ({ page }) => {
  await login(page);
  await page.goto('/admin/lessons');
  await expect(page.getByRole('button', { name: 'New course' })).toBeVisible();
  // Every lessons RPC is refused; /v1/auth/refresh still succeeds.
  await page.route('**/lessons.v1.LessonsService/**', (route) => route.fulfill(unauthenticatedRPC));
  await page.getByRole('textbox', { name: 'Search title…' }).fill('x');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await page.waitForURL(/\/login$/, { timeout: 10_000 });
});

test('REST call still refused after a successful refresh signs out to /login', async ({ page }) => {
  await login(page);
  await page.goto('/admin/tags');
  await expect(page.getByRole('button', { name: 'New tag' })).toBeVisible();
  await page.route('**/v1/admin/tags**', (route) => route.fulfill(unauthenticatedREST));
  await page.getByRole('textbox', { name: 'Search name or slug…' }).fill('x');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await page.waitForURL(/\/login$/, { timeout: 10_000 });
});

test('media upload refused as unauthenticated signs out to /login', async ({ page }) => {
  await login(page);
  await page.goto('/admin/media');
  await page.route('**/v1/admin/media', (route) =>
    route.request().method() === 'POST' ? route.fulfill(unauthenticatedREST) : route.fallback()
  );
  await page.locator('input[type="file"]').setInputFiles({
    name: 'pixel.png',
    mimeType: 'image/png',
    buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64'),
  });
  await page.waitForURL(/\/login$/, { timeout: 10_000 });
});

test('a call refused once but rescued by the refresh stays signed in', async ({ page }) => {
  await login(page);
  await page.goto('/admin/lessons');
  await expect(page.getByRole('button', { name: 'New course' })).toBeVisible();
  let refused = false;
  await page.route('**/lessons.v1.LessonsService/ListCourses', (route) => {
    if (refused) return route.fallback();
    refused = true;
    return route.fulfill(unauthenticatedRPC);
  });
  await page.getByRole('textbox', { name: 'Search title…' }).fill('Go');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('link', { name: 'Go for a Laravel engineer' })).toBeVisible();
  expect(refused).toBe(true);
  await expect(page).toHaveURL(/\/admin\/lessons$/);
});
