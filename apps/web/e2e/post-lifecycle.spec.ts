import { test, expect, type Page } from '@playwright/test';

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';
const API_BASE = process.env.E2E_API_BASE ?? 'http://localhost:8080';

function randomSuffix() {
  return Math.random().toString(36).slice(2, 8);
}

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
}

test('post lifecycle: create category, create post, publish, verify, delete', async ({
  page,
  request,
}) => {
  const suffix = randomSuffix();
  const categoryName = `e2e-cat-${suffix}`;
  const postTitle = `e2e-post-${suffix}`;
  const postSlug = postTitle; // slugify of e2e-post-xxxxxx is identical

  await login(page);

  // --- Step 1: Create category ---
  await page.goto('/admin/categories');
  await expect(page.getByRole('heading', { name: 'Categories' })).toBeVisible();

  await page.getByRole('button', { name: 'New category' }).click();

  const categoryDialog = page.getByRole('dialog');
  await expect(categoryDialog.getByText('New category')).toBeVisible();
  await categoryDialog.getByPlaceholder('Category name').fill(categoryName);
  await categoryDialog.getByRole('button', { name: 'Create' }).click();

  await expect(categoryDialog).toBeHidden({ timeout: 10_000 });
  // Both Name and Slug columns render the same text — use .first() to avoid
  // strict-mode violations.
  await expect(
    page.getByRole('cell', { name: categoryName, exact: true }).first()
  ).toBeVisible({ timeout: 10_000 });

  // --- Step 2: Create post ---
  await page.goto('/admin/posts/new');
  await expect(page.getByRole('heading', { name: 'New post' })).toBeVisible();

  await page.getByPlaceholder('Post title').fill(postTitle);
  await page.getByPlaceholder('Write your post…').fill('Hello E2E');

  // Category combobox shows "Uncategorized" by default. Locate it by current
  // displayed text and pick our newly-created category.
  await page.getByRole('combobox').filter({ hasText: 'Uncategorized' }).click();
  await page.getByRole('option', { name: categoryName, exact: true }).click();

  // Status defaults to "draft" already, no change needed.
  await page.getByRole('button', { name: 'Create' }).click();

  // After creation we're redirected to /admin/posts/<id>
  await page.waitForURL(/\/admin\/posts\/[0-9a-f-]{36}/, { timeout: 15_000 });
  await expect(page.getByRole('heading', { name: 'Edit post' })).toBeVisible();

  // --- Step 3: Publish ---
  await page.getByRole('button', { name: 'Publish' }).click();
  // Either toast "Post published" appears or the Publish button becomes Unpublish.
  await expect(
    page.getByRole('button', { name: 'Unpublish' })
  ).toBeVisible({ timeout: 10_000 });

  // --- Step 4: Verify via public API ---
  // The public endpoint returns ONLY published posts — a 200 here is proof of
  // publication. The response shape does not include `status` (admin-only field).
  const apiResponse = await request.get(`${API_BASE}/v1/posts/${postSlug}`);
  expect(apiResponse.status()).toBe(200);
  const body = (await apiResponse.json()) as {
    data?: { slug?: string; published_at?: string | null };
  };
  const post = body.data;
  expect(post?.slug).toBe(postSlug);
  expect(post?.published_at).toBeTruthy();

  // --- Step 5: Cleanup — delete post via UI ---
  await page.getByRole('button', { name: 'Delete' }).click();
  const postDeleteDialog = page.getByRole('dialog');
  await expect(postDeleteDialog.getByText('Delete post?')).toBeVisible();
  await postDeleteDialog.getByRole('button', { name: 'Delete' }).click();
  await page.waitForURL(/\/admin\/posts(\?.*)?$/, { timeout: 15_000 });

  // --- Step 6: Cleanup — delete category via UI ---
  await page.goto('/admin/categories');
  const categoryRow = page.getByRole('row', { name: new RegExp(categoryName) });
  await categoryRow.getByRole('button', { name: 'Actions' }).click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();

  const catDeleteDialog = page.getByRole('dialog');
  await expect(catDeleteDialog.getByText('Delete category?')).toBeVisible();
  await catDeleteDialog.getByRole('button', { name: 'Delete' }).click();
  await expect(catDeleteDialog).toBeHidden({ timeout: 10_000 });
  await expect(
    page.getByRole('cell', { name: categoryName, exact: true })
  ).toHaveCount(0, { timeout: 10_000 });
});
