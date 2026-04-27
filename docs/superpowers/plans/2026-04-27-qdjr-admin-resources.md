# qdjr-admin Resources + Admin UI — Implementation Plan (Plan 2 of 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver every public and admin endpoint in the spec, plus a working Next.js admin UI that authenticates against Supabase, lists/creates posts, manages taxonomy and media, and edits profile + site settings.

**Architecture:** Every resource lives in its own Go package under `apps/api/internal/<resource>/` with three layers: `repository.go` (pgx queries), `service.go` (business logic + RBAC ownership checks), `handler.go` (Gin handlers). Public endpoints are registered in a single `internal/publicapi` package and admin endpoints in `internal/adminapi` (already exists). The Web UI is a Next.js 15 App Router app at `apps/web/` using shadcn/ui, TanStack Query, react-hook-form + zod, and a typed fetch client that talks to the Go BE.

**Tech additions:** golang.org/x/time/rate (rate limiting), pnpm + Next.js 15 + TypeScript + shadcn/ui + TanStack Query + zod + react-hook-form.

**Spec:** `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`. Plan 1 already shipped at tag `v0.1.0-foundation`.

---

## Conventions for every resource task

Each resource task creates files in `apps/api/internal/<resource>/`:

- `<resource>.go` — typed model struct (DB row representation)
- `repository.go` — pgx queries; one method per access pattern; uses `pgxpool.Pool`
- `service.go` — orchestrates repo + RBAC ownership for `author` role; pure functions where possible
- `handler.go` — `RegisterPublic(g)` and/or `RegisterAdmin(g, deps)`; uses `apphttp.OK/List/Err`
- Tests:
  - `repository_test.go` — integration test against local Supabase via `TEST_DATABASE_URL`. Use `uuid.New()` for test data so reruns don't collide.
  - `handler_test.go` — `httptest` against a stub repo or real DB depending on the test
- Wire up: `cmd/api/main.go` adds the `RegisterPublic` / `RegisterAdmin` callbacks

Response shapes match `qdjr/plugins/api.ts:16-47` for public endpoints. Admin endpoints follow the standard envelope with `Meta` for paginated lists.

Default pagination: `perPage=20`, max 100.

---

## Phase A — Public API (Tasks 1-5)

### Task 1: Posts public — `GET /v1/posts`, `GET /v1/posts/:slug`

**Files:** `apps/api/internal/posts/{posts.go, repository.go, repository_test.go, handler.go, handler_test.go}`, modify `cmd/api/main.go`.

**Behavior:**
- `GET /v1/posts?page=&perPage=&category=&tag=&q=` — only `published`; sort by `published_at desc`. Joins categories + tags + thumbnail.
- `GET /v1/posts/:slug` — single post; 404 envelope if missing or non-published.
- Response per post: `id, slug, title, excerpt, content, published_at, thumbnail (url+alt), location (NULL — kept for FE compat), category (first if any), tags[], created_at, updated_at`.
- Uses `apphttp.List` with `Meta{page, perPage, total}` and `apphttp.OK`.

**TDD:**
1. Write `repository_test.go` with two tests: `ListPublished_PagesAndFilters` and `GetPublishedBySlug_ReturnsErrNotFound`. Use `uuid.New()` test data; insert via Supabase service-role; clean up in `t.Cleanup`. Test expects `ErrNotFound` sentinel from `posts` package.
2. Run — should fail compile.
3. Implement `posts.go` (struct types), `repository.go` with `ListPublished(ctx, ListFilter) ([]Post, int, error)` and `GetPublishedBySlug(ctx, slug) (Post, error)`. Define `var ErrNotFound = errors.New("not found")`.
4. Run repo tests with `TEST_DATABASE_URL` — green.
5. Write `handler_test.go` with stub repo; tests for 200 list, 200 single, 404 missing.
6. Implement `handler.go` `RegisterPublic(g, repo)`.
7. Wire in `cmd/api/main.go`: build a `posts.NewRepository(pool)`, pass to `RegisterPublic` for the public group.
8. Run all tests; commit `feat(api/posts): add public list and slug endpoints`.

### Task 2: Categories public — `GET /v1/categories`, `GET /v1/categories/:slug/posts`

**Files:** `apps/api/internal/categories/{categories.go, repository.go, repository_test.go, handler.go, handler_test.go}`.

`/v1/categories` returns `[{id, slug, name, description}]` (no envelope meta — fixed-size list). `/v1/categories/:slug/posts` reuses the posts listing with a category filter and pagination.

Same TDD pattern: repo test → impl → handler test → impl → wire → commit `feat(api/categories): add public list and posts-by-category endpoints`.

### Task 3: Tags public — `GET /v1/tags`, `GET /v1/tags/:slug/posts`

Mirror of Task 2 with `tags`. Commit `feat(api/tags): add public list and posts-by-tag endpoints`.

### Task 4: Profile + Site Settings public — `GET /v1/profile`, `GET /v1/site-settings`

**Files:** `apps/api/internal/profile/{profile.go, repository.go, repository_test.go, handler.go, handler_test.go}`, same shape for `site_settings/`.

Singleton resources (id=1). `GET` returns the row; if absent returns 404 (should never happen since seed inserts).

`GET /v1/site-settings` redacts `contact_email` (admin-only field). Public response: `{site_title, site_description, footer_text, social_links}`.

Two commits — one per package: `feat(api/profile): add public GET endpoint`, `feat(api/site_settings): add public GET endpoint`.

### Task 5: Contact form — `POST /v1/contact` with rate limit

**Files:** `apps/api/internal/contact/{contact.go, repository.go, repository_test.go, handler.go, handler_test.go, ratelimit.go, ratelimit_test.go}`.

- Rate limit: per-IP token bucket via `golang.org/x/time/rate`. 5 requests / hour / IP. In-memory (sync.Map of `*rate.Limiter`); evict idle limiters every 10 min via background goroutine started by `RegisterPublic`.
- Validation: `name 1-200`, `email valid + ≤200`, `subject ≤200`, `body 1-5000`. Reject with 400 `VALIDATION` envelope on failure.
- IP extraction order: `X-Forwarded-For` first hop → `X-Real-IP` → `RemoteAddr`. Trim whitespace.
- Insert into `contact_messages(status='new', ip=<inet>, user_agent=<header>)`. Return 201 with `{id, created_at}`.

TDD: ratelimit tests (allow first 5, reject 6th, separate IPs are independent), validation tests, repo insert test, handler test (200 happy + 429 sixth + 400 validation).

Commit `feat(api/contact): add rate-limited public contact endpoint`.

---

## Phase B — Admin API (Tasks 6-13)

### Task 6: Posts admin — full CRUD + publish/unpublish + RBAC

**Files:** Extend `apps/api/internal/posts/` with `repository_admin.go`, `service.go`, `handler_admin.go`, `*_test.go`.

Endpoints:
- `GET /v1/admin/posts?page=&perPage=&status=&q=` — `posts:read:all` for super_admin/editor (any author), or own-only for `author` role (filter by `created_by = uid`).
- `POST /v1/admin/posts` — `posts:write` (any role). Body: title, slug?, excerpt?, content, status (draft|published), thumbnail_id?, og_image_id?, meta_*?, category_ids[], tag_ids[]. Auto-generate slug from title if missing (lowercase, hyphenated). Set `created_by = uid`. If `status = published`, require `posts:publish` perm; if author lacks it, force `status = draft`.
- `GET /v1/admin/posts/:id` — `posts:read:all` OR (own + author).
- `PATCH /v1/admin/posts/:id` — `posts:write` + ownership for author. Partial updates; revalidate slug uniqueness if slug changes. Updates `updated_by` and `updated_at`.
- `DEL /v1/admin/posts/:id` — `posts:write` + ownership for author. Hard delete (cascade pivots).
- `POST /v1/admin/posts/:id/publish` — `posts:publish`. Sets `status='published'`, `published_at=now()` if null.
- `POST /v1/admin/posts/:id/unpublish` — `posts:publish`. Sets `status='draft'`.

Service layer enforces ownership: `service.RequireOwnedByAuthor(ctx, postID, uid, role, resolver)` returns 403 if author and not owner.

TDD: repo tests for create/get/update/delete + tag/category links; service test for ownership branch; handler tests for each endpoint with stub service.

Commit `feat(api/posts): add admin CRUD with RBAC and ownership enforcement`.

### Task 7: Categories admin — full CRUD

`GET|POST /v1/admin/categories`, `GET|PATCH|DEL /v1/admin/categories/:id` — `taxonomy:write`. Slug uniqueness + auto-generate from name. `DELETE` blocked if any post references the category (return 409 `CONFLICT`).

Commit `feat(api/categories): add admin CRUD`.

### Task 8: Tags admin — full CRUD

Mirror of Task 7. Commit `feat(api/tags): add admin CRUD`.

### Task 9: Media admin — list, signed-url, register, delete

**Files:** `apps/api/internal/media/{media.go, repository.go, storage.go, handler.go, *_test.go}`.

- `GET /v1/admin/media?page=&perPage=` — `media:write`. Returns paginated list.
- `POST /v1/admin/media/signed-url` — body `{filename, mime_type, size}`. Validates mime is in allowlist (`image/png|jpeg|webp|gif`, max 10 MB). Generates `storage_path = "media/" + uuid + ext`. Returns Supabase Storage signed upload URL via the storage service-role API: `POST /storage/v1/object/upload/sign/<bucket>/<path>`. The bucket is `media` (created if missing on first call — implement `EnsureBucket` once at startup).
- `POST /v1/admin/media` — body `{filename, storage_path, mime_type, size, width?, height?, alt_text?}`. Inserts the `media` row with `uploaded_by = uid`. The UI uploads first via signed URL, then calls this to register.
- `DEL /v1/admin/media/:id` — deletes both the storage object (via Supabase Storage REST) and the `media` row. `media:write` + ownership for author.

Add `internal/storage/` package with `Client.SignedUploadURL(path string) (string, error)` and `Client.Delete(path string) error`, both calling the Supabase REST API with the service-role key.

Commit `feat(api/media): add admin list, signed-url upload, register, delete`.

### Task 10: Profile admin — `GET|PATCH /v1/admin/profile`

`profile:write`. PATCH is partial. Sets `updated_by`, `updated_at`. Commit `feat(api/profile): add admin GET and PATCH endpoints`.

### Task 11: Site Settings admin — `GET|PATCH /v1/admin/site-settings`

`settings:write`. PATCH partial. Commit `feat(api/site_settings): add admin GET and PATCH endpoints`.

### Task 12: Contact messages admin — `GET`, `PATCH /v1/admin/contact-messages/:id`

`contact:read` to list. `contact:write` to update status. `GET` supports `?status=new|read|replied|spam&page=&perPage=`. `PATCH` only updates `status` field. Commit `feat(api/contact): add admin list and status-update endpoints`.

### Task 13: Users admin — list, invite, change role, delete

**Files:** `apps/api/internal/users/{users.go, repository.go, supabase_admin.go, handler.go, *_test.go}`.

- `GET /v1/admin/users?page=&perPage=` — `users:manage`. Joins `auth.users` + `public.user_roles` + `public.roles`. Returns `[{id, email, role, last_sign_in_at, assigned_at}]`.
- `POST /v1/admin/users` — body `{email, role, password?}`. `users:manage`. Reuses bootstrap CLI's `ensureAuthUser` logic via the new `users.SupabaseAdminClient`. Inserts the role row. Returns 201 with the user shape.
- `PATCH /v1/admin/users/:id/role` — body `{role}`. `users:manage`. Updates `user_roles.role_id`. Cannot change own role to non-super_admin if current user is the last super_admin (return 409).
- `DEL /v1/admin/users/:id` — `users:manage`. Deletes from `auth.users` (cascades to `user_roles`). Same self-protection as PATCH.

Commit `feat(api/users): add admin user management endpoints`.

---

## Phase C — Admin Web UI (Tasks 14-23)

### Task 14: Next.js 15 scaffolding

**Files:** `apps/web/{package.json, tsconfig.json, next.config.ts, postcss.config.mjs, tailwind.config.ts, .env.example, .env.local, app/layout.tsx, app/globals.css}` plus `apps/web/components/ui/*` from shadcn-init.

Steps:
1. `cd apps/web && pnpm create next-app@latest . --typescript --tailwind --eslint --app --no-src-dir --import-alias "@/*"` — answer non-interactive flags.
2. Install: `pnpm add @tanstack/react-query @tanstack/react-table react-hook-form zod @hookform/resolvers @supabase/supabase-js sonner lucide-react`.
3. `pnpm dlx shadcn@latest init` (defaults: New York, slate, RSC, prefix none) then `pnpm dlx shadcn@latest add button input label form table dialog dropdown-menu select textarea card sonner skeleton badge tabs sheet`.
4. Edit `app/layout.tsx` to wire `QueryClientProvider` (in a `'use client'` `app/providers.tsx`) and `<Toaster />`.
5. `apps/web/.env.example`: `NEXT_PUBLIC_API_URL=http://localhost:8080`, `NEXT_PUBLIC_SUPABASE_URL=http://127.0.0.1:54321`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=<sb_publishable_*>`.

Commit `chore(web): scaffold Next.js 15 + shadcn/ui + tanstack stack`.

### Task 15: Supabase Auth client + login page

**Files:** `apps/web/lib/supabase.ts`, `apps/web/lib/auth.ts`, `apps/web/app/(auth)/login/page.tsx`, `apps/web/app/(auth)/login/login-form.tsx`.

- `lib/supabase.ts`: `createBrowserClient` with publishable key.
- `lib/auth.ts`: `signIn(email, password)`, `signOut()`, `getAccessToken()` (reads from current session).
- Login page: card + form (email + password) using react-hook-form + zod. On success, push to `/admin`. On failure, show toast.

Commit `feat(web/auth): add Supabase login page`.

### Task 16: API client wrapper (typed fetch with JWT)

**Files:** `apps/web/lib/api.ts`.

Single `apiFetch<T>(path, init?)` that:
- Reads JWT via `lib/auth.getAccessToken()`.
- Sends `Authorization: Bearer <jwt>`, `Content-Type: application/json`.
- Wraps fetch with `NEXT_PUBLIC_API_URL` base.
- Throws `ApiError {status, code, message}` on non-2xx using the response envelope.
- Generic helpers: `apiGet<T>(path)`, `apiPost<T>(path, body)`, `apiPatch<T>(path, body)`, `apiDelete(path)`, `apiList<T>(path)` — last returns `{data: T[], meta}`.

Plus shared `types.ts` with `Post`, `Category`, `Tag`, `Media`, `Profile`, `SiteSettings`, `ContactMessage`, `User` interfaces matching the API response shapes.

Commit `feat(web/lib): add typed API client wrapper`.

### Task 17: Protected admin layout

**Files:** `apps/web/app/admin/layout.tsx`, `apps/web/app/admin/sidebar.tsx`, `apps/web/app/admin/user-menu.tsx`, `apps/web/middleware.ts`.

- Middleware checks Supabase session; redirects unauthenticated to `/login` for any `/admin/*`.
- Layout: sidebar with links (Posts, Categories, Tags, Media, Profile, Settings, Inbox, Users), top-right user menu with sign-out and "current role" badge fetched from `/v1/admin/me`.
- Use shadcn `Sheet` for mobile sidebar.

Commit `feat(web/admin): add protected layout with sidebar and user menu`.

### Task 18: Posts list + create/edit page

**Files:** `apps/web/app/admin/posts/page.tsx`, `apps/web/app/admin/posts/[id]/page.tsx`, `apps/web/app/admin/posts/new/page.tsx`, `apps/web/app/admin/posts/post-form.tsx`, `apps/web/app/admin/posts/posts-table.tsx`.

- List page: TanStack Table with columns `title, slug, status (badge), category, published_at`, server-paginated via TanStack Query (`useQuery({queryKey: ['posts', page, q, status], queryFn})`). Filters: status select, search input, category select.
- New/edit page: react-hook-form + zod schema; fields title, slug (auto-fill from title), excerpt, content (textarea — Markdown for now; keep simple), status, category (single select), tags (multiselect), thumbnail (`MediaPicker` from Task 20), publish date.
- Actions: Save Draft, Publish, Unpublish, Delete.

Commit `feat(web/admin): add posts list and create/edit pages`.

### Task 19: Categories + tags pages

**Files:** `apps/web/app/admin/categories/page.tsx`, `apps/web/app/admin/categories/category-form.tsx`, mirror for tags.

Simple list (table) + create/edit dialog. Inline edit via dialog. Delete with confirmation.

Commit `feat(web/admin): add categories and tags management pages`.

### Task 20: Media gallery with upload

**Files:** `apps/web/app/admin/media/page.tsx`, `apps/web/components/media-picker.tsx`, `apps/web/lib/upload.ts`.

- Gallery: grid of thumbnails + filename + size; click for details panel; delete button per item.
- Upload: drag/drop or click. `lib/upload.ts` calls BE `/v1/admin/media/signed-url`, PUTs file to that URL, then POSTs to `/v1/admin/media` to register.
- `MediaPicker`: reusable dialog component used by post form.

Commit `feat(web/admin): add media gallery with signed-URL upload`.

### Task 21: Profile + site settings forms

**Files:** `apps/web/app/admin/profile/page.tsx`, `apps/web/app/admin/settings/page.tsx`.

Forms with all singleton fields, react-hook-form + zod. Save via PATCH. Toast on success.

Commit `feat(web/admin): add profile and site-settings editors`.

### Task 22: Contact inbox

**Files:** `apps/web/app/admin/contact/page.tsx`.

Tabs: New / Read / Replied / Spam. Click row to view full message in a Sheet. Status update buttons (Mark read, Mark spam, Mark replied) via PATCH.

Commit `feat(web/admin): add contact-messages inbox`.

### Task 23: Users management

**Files:** `apps/web/app/admin/users/page.tsx`, `apps/web/app/admin/users/invite-form.tsx`.

Table of users with email, role badge, last sign-in. Invite dialog: email + role + temp password. Per-row dropdown: Change role (super_admin/editor/author), Delete user. `users:manage` gate (server returns 403 for non-super_admin; UI hides menu items based on `/v1/admin/me` role).

Commit `feat(web/admin): add users management page`.

---

## Phase D — Verification (Task 24)

### Task 24: End-to-end smoke test

**Files:** `apps/web/playwright.config.ts`, `apps/web/e2e/login.spec.ts`, `apps/web/e2e/post-lifecycle.spec.ts`.

- Login: navigate to `/login`, fill credentials, expect redirect to `/admin`.
- Post lifecycle: log in → create category "test" → upload an image → create post with title, content, that category, that thumbnail → save as draft → click publish → call BE public `/v1/posts/:slug` and assert status published.

Add `pnpm test:e2e` script. Use the bootstrap user from Plan 1.

Commit `feat(web/e2e): add login and post-lifecycle Playwright tests`. Tag `v0.2.0-resources`.

---

## Verification — Plan 2 acceptance

1. `cd apps/api && go test -race -cover ./...` green; coverage ≥ 70% on every resource handler package.
2. `cd apps/web && pnpm test` (Vitest) and `pnpm test:e2e` (Playwright) green.
3. With local Supabase + bootstrap admin, the full create-image-upload → create-post → publish → fetch-via-public-API flow works.
4. qdjr's `plugins/api.ts:57-115` shape matches what `/v1/posts` returns (verify by curl + jq-diff against the existing TS interfaces).
5. RBAC enforced server-side: `author` cannot publish; `editor` cannot manage users; both verified by hitting the API with crafted JWTs in handler tests.

## Out of scope

- Custom rich-text editor (Markdown textarea is sufficient for Plan 2)
- Image resizing / variants (use original uploads; defer to Plan 3+ if needed)
- Search beyond `q` substring filter
- Audit log
