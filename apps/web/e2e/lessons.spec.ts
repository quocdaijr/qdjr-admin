import { test, expect, type Page } from '@playwright/test';

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';

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

test('lessons: create course, add lesson, mark done, progress updates, delete course', async ({ page }) => {
  const suffix = randomSuffix();
  const courseTitle = `e2e-course-${suffix}`;
  const lessonTitle = `e2e-lesson-${suffix}`;

  await login(page);

  await page.goto('/admin/lessons');
  await expect(page.getByRole('heading', { name: 'Lessons' })).toBeVisible();

  await page.getByRole('button', { name: 'New course' }).click();
  const courseDialog = page.getByRole('dialog');
  await courseDialog.getByLabel('Title').fill(courseTitle);
  await courseDialog.getByLabel('Provider').fill('e2e');
  await courseDialog.getByRole('button', { name: 'Create' }).click();
  await expect(courseDialog).toBeHidden();

  const row = page.getByRole('row', { name: new RegExp(courseTitle) });
  await expect(row).toContainText('0/0');
  await row.getByRole('link', { name: courseTitle }).click();
  await expect(page.getByRole('heading', { name: courseTitle })).toBeVisible();

  await page.getByRole('button', { name: 'New lesson' }).click();
  const lessonDialog = page.getByRole('dialog');
  await lessonDialog.getByLabel('Title').fill(lessonTitle);
  await lessonDialog.getByRole('button', { name: 'Add' }).click();
  await expect(lessonDialog).toBeHidden();
  await expect(page.getByText(lessonTitle)).toBeVisible();
  await expect(page.getByText('0/1 done')).toBeVisible();

  await page.getByRole('combobox', { name: `Status of ${lessonTitle}` }).click();
  await page.getByRole('option', { name: 'Done' }).click();
  await expect(page.getByText('1/1 done')).toBeVisible();

  // Cleanup through the UI so the test leaves no rows behind.
  await page.goto('/admin/lessons');
  const rowAgain = page.getByRole('row', { name: new RegExp(courseTitle) });
  await expect(rowAgain).toContainText('1/1');
  await rowAgain.getByRole('button', { name: 'Actions' }).click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
  await expect(page.getByRole('row', { name: new RegExp(courseTitle) })).toHaveCount(0);
});

test('lessons: server field error shows under the title input', async ({ page }) => {
  await login(page);
  await page.goto('/admin/lessons');
  await page.getByRole('button', { name: 'New course' }).click();
  const dialog = page.getByRole('dialog');
  // 201 characters: only the server rejects it (google.rpc.BadRequest detail).
  await dialog.getByLabel('Title').fill('a'.repeat(201));
  await dialog.getByRole('button', { name: 'Create' }).click();
  await expect(dialog.getByText('title must be 1..200 characters')).toBeVisible();
  await expect(dialog).toBeVisible();
});

test('lessons: reorder with the up and down buttons', async ({ page }) => {
  const suffix = randomSuffix();
  const courseTitle = `e2e-reorder-${suffix}`;
  await login(page);
  await page.goto('/admin/lessons');
  await page.getByRole('button', { name: 'New course' }).click();
  await page.getByRole('dialog').getByLabel('Title').fill(courseTitle);
  await page.getByRole('dialog').getByRole('button', { name: 'Create' }).click();
  await page.getByRole('link', { name: courseTitle }).click();

  for (const t of ['first', 'second']) {
    await page.getByRole('button', { name: 'New lesson' }).click();
    await page.getByRole('dialog').getByLabel('Title').fill(t);
    await page.getByRole('dialog').getByRole('button', { name: 'Add' }).click();
    await expect(page.getByRole('dialog')).toBeHidden();
  }
  const rows = page.locator('tbody tr');
  await expect(rows.nth(0)).toContainText('first');
  await expect(page.getByRole('button', { name: 'Move first up' })).toBeDisabled();

  await page.getByRole('button', { name: 'Move second up' }).click();
  await expect(rows.nth(0)).toContainText('second');
  await expect(rows.nth(1)).toContainText('first');

  // Survives a reload: the order is stored, not just rendered.
  await page.reload();
  await expect(rows.nth(0)).toContainText('second');

  await page.goto('/admin/lessons');
  const row = page.getByRole('row', { name: new RegExp(courseTitle) });
  await row.getByRole('button', { name: 'Actions' }).click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
  await expect(page.getByRole('row', { name: new RegExp(courseTitle) })).toHaveCount(0);
});

test('lessons: status change is optimistic and rolls back on failure', async ({ page }) => {
  const suffix = randomSuffix();
  const courseTitle = `e2e-optimistic-${suffix}`;
  const lessonTitle = `e2e-opt-lesson-${suffix}`;
  await login(page);
  await page.goto('/admin/lessons');
  await page.getByRole('button', { name: 'New course' }).click();
  await page.getByRole('dialog').getByLabel('Title').fill(courseTitle);
  await page.getByRole('dialog').getByRole('button', { name: 'Create' }).click();
  await page.getByRole('link', { name: courseTitle }).click();
  await page.getByRole('button', { name: 'New lesson' }).click();
  await page.getByRole('dialog').getByLabel('Title').fill(lessonTitle);
  await page.getByRole('dialog').getByRole('button', { name: 'Add' }).click();
  await expect(page.getByRole('dialog')).toBeHidden();

  // Hold the server's answer for 1.5s, then fail it.
  await page.route('**/lessons.v1.LessonsService/UpdateLesson', async (route) => {
    await new Promise((r) => setTimeout(r, 1500));
    await route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ code: 'unavailable', message: 'api down' }),
    });
  });
  // The API stays down for the resync too: a failed background refetch must
  // keep the cached course on screen, not replace the page with an error.
  await page.route('**/lessons.v1.LessonsService/GetCourse', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ code: 'unavailable', message: 'api down' }),
    })
  );
  const trigger = page.getByRole('combobox', { name: `Status of ${lessonTitle}` });
  await trigger.click();
  await page.getByRole('option', { name: 'Done' }).click();

  // Shown before the server answers...
  await expect(trigger).toContainText('Done', { timeout: 1000 });
  await expect(page.getByText('1/1 done')).toBeVisible({ timeout: 1000 });
  // ...and rolled back with an error once it fails.
  await expect(page.getByText('Unavailable: api down')).toBeVisible({ timeout: 5000 });
  await expect(trigger).toContainText('To do');
  await expect(page.getByText('0/1 done')).toBeVisible();
  // Give the failed resync time to land, then the row must still be there.
  await page.waitForTimeout(1000);
  await expect(page.getByRole('row', { name: new RegExp(lessonTitle) })).toBeVisible();

  await page.unroute('**/lessons.v1.LessonsService/UpdateLesson');
  await page.unroute('**/lessons.v1.LessonsService/GetCourse');
  await page.goto('/admin/lessons');
  const row = page.getByRole('row', { name: new RegExp(courseTitle) });
  await row.getByRole('button', { name: 'Actions' }).click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
  await expect(page.getByRole('row', { name: new RegExp(courseTitle) })).toHaveCount(0);
});
