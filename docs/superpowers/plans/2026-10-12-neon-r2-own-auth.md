# qdjr-admin: Supabase → Neon + Cloudflare R2 + own JWT auth (pate-shop pattern)

> Executed on `feature/dainq/goose-render-cloudflare-site-cutover` (extends PR #1). Nothing was ever deployed, so the Supabase project is never set up.

**Goal:** Remove every Supabase dependency. Postgres moves to Neon, media to Cloudflare R2, and admin auth becomes the API's own JWT access/refresh tokens with bcrypt passwords — the same pattern `pate-shop` runs in production.

**Decisions (user, 2026-10-12):**
1. Rewrite migrations 0002–0004 in place to reference a new `public.users` table. Allowed once: no database outside a laptop ever applied them.
2. Media on Cloudflare R2.
3. Admins are created with an initial password set by a super_admin; the first one is seeded from env at API start. No email provider.
4. Land in PR #1.

## Architecture

| Concern | Before | After | Source pattern |
|---|---|---|---|
| Database | Supabase Postgres | Neon. `DATABASE_URL` = pooled URL (`-pooler` host, `sslmode=require`). `MIGRATIONS_DATABASE_URL` = direct URL (optional, falls back to `DATABASE_URL`) | pate-shop `DB_DSN` + improvement: migrations off the pooler |
| First connect | single ping | 5 attempts with backoff (Neon wakes a suspended compute on connect) | pate-shop research doc TODO |
| Auth | supabase-js + JWKS | `POST /v1/auth/login`, `POST /v1/auth/refresh` (public, per-IP limited). HS256, `JWT_SECRET`, claims `sub`, `email`, `typ` (`access`/`refresh`). Access 15 m, refresh 7 d. bcrypt hashes in `public.users`. Refresh re-reads the user, so a deleted user cannot refresh | pate-shop admin realm |
| Password change | Supabase | `PATCH /v1/admin/me/password {current_password, new_password}` + dialog in the user menu | new (no email reset) |
| Users | Supabase Admin API | `users.PGAccounts` (create with bcrypt, delete row; FKs cascade) | — |
| Bootstrap | `cmd/bootstrap` calling Supabase | API seeds `BOOTSTRAP_ADMIN_EMAIL`/`BOOTSTRAP_ADMIN_PASSWORD` as super_admin at start if that email has no user (Render free has no shell) | pate-shop `SEED_OWNER_*` |
| Media | browser PUT to Supabase signed URL, client-declared mime | `POST /v1/admin/media` multipart; server sniffs type (`http.DetectContentType`), allows png/jpeg/webp/gif ≤ 10 MB, reads dimensions, stores `media/<uuid>.<ext>` | pate-shop upload-through-API |
| Object store | Supabase Storage | `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET` via aws-sdk-go-v2 s3 (`Region: auto`, path style). Local dev without R2: filesystem driver under `MEDIA_DIR`, served by the API at `/media-files/`. `ENV=production` refuses to start without R2 | pate-shop `r2.go` |
| Public media URL | computed from Supabase URL | `MEDIA_PUBLIC_BASE_URL` + `/` + `storage_path`, computed at read time (r2.dev now; custom domain later = env change only) | improvement over pate-shop (stores full URL) |
| Local DB | `supabase start` | `docker-compose.yml` → `postgres:18-alpine` on 5432 | pate-shop |
| CI | DB tests skipped | `api.yml` runs a `postgres:18-alpine` service, migrates, runs DB-backed tests | pate-shop `backend-migrations` job |

**Security notes:** SVG uploads are dropped (script-capable); file type comes from bytes, not the client. Login and refresh share a per-IP limiter (10/min). Passwords ≥ 10 chars. `JWT_SECRET` must be ≥ 32 bytes. Tokens live in `localStorage` (same trade-off as pate-shop; admin is a separate origin with no third-party scripts).

**Not changed:** public `/v1/*` contract, RBAC model, site, Render/Cloudflare/Netlify topology, goose-at-startup.

## File map

| Path | Change |
|---|---|
| `apps/api/migrations/0002_rbac.sql` | create `public.users` first; `user_roles.user_id` → `public.users` |
| `apps/api/migrations/0003_content.sql`, `0004_singletons.sql` | FK targets `auth.users` → `public.users` |
| `apps/api/internal/auth/{jwt.go,password.go,handler.go,limiter.go}` | issuer/verifier, bcrypt, login/refresh/password handlers |
| `apps/api/internal/users/{accounts.go,seed.go,...}` | `PGAccounts`, `SeedAdmin`; repository on `public.users`; delete `supabase_admin*.go` |
| `apps/api/internal/media/{storage.go,r2.go,localfs.go,handler_admin.go,admin.go}` | `Storage{Put,Delete}`, R2 + local drivers, multipart upload |
| `apps/api/internal/config/config.go`, `internal/db/pool.go`, `cmd/api/main.go` | new env, retry, wiring; delete `cmd/bootstrap` |
| `apps/api/**/*_test.go` | fixtures insert into `public.users` |
| `docker-compose.yml` (root), delete `supabase/` | local Postgres |
| `apps/web/lib/{auth.ts,api.ts}`, delete `lib/supabase.ts` | token storage, refresh, events |
| `apps/web/app/admin/media/upload-dropzone.tsx`, `users/invite-dialog.tsx`, `user-menu.tsx` (+ `change-password-dialog.tsx`) | UI for the new flows |
| `render.yaml`, `.github/workflows/{api,web}.yml`, env examples, docs | new env names, CI Postgres |

## Tasks (TDD; one commit each)

1. **Schema + local Postgres.** Rewrite 0002–0004; root `docker-compose.yml`; delete `supabase/`; retarget all test fixtures. *Done when:* fresh `migrate up` reaches 9 on postgres:18; full `go test -race ./...` green with `TEST_DATABASE_URL`.
2. **Own auth (API).** `auth.Issuer`, HS256 verifier rejecting `typ=refresh`, bcrypt helpers, login/refresh/password handlers + limiter, `users.PGAccounts`, `SeedAdmin`, config (`JWT_SECRET` ≥ 32, TTLs, bootstrap vars), drop JWKS + `keyfunc`. *Done when:* unit tests cover wrong password, unknown email (same error), refresh with access token rejected, access with refresh token rejected, deleted user cannot refresh, short password rejected; seed is idempotent.
3. **Media on R2 (API).** `Storage` interface, R2 driver (unit-tested against an httptest S3 endpoint), local fs driver + `/media-files/` route, multipart handler with sniffing, production guard. *Done when:* tests cover png accepted with dimensions, svg/text rejected, >10 MB rejected (413), storage error → 502 and no DB row.
4. **Neon wiring.** `DATABASE_URL`/`MIGRATIONS_DATABASE_URL`, first-connect retry, `render.yaml`, env examples. *Done when:* API boots against local Postgres via both URLs; retry unit-tested.
5. **Admin UI.** `lib/auth.ts` on API tokens (single-flight refresh, 401 retry once, cross-tab sign-out), remove `@supabase/supabase-js`, dropzone multipart with progress, invite requires password, change-password dialog, env/web.yml cleanup. *Done when:* tsc/lint/build green; Playwright login, post lifecycle, profile content, user menu pass; new e2e for change password.
6. **CI + docs.** Postgres service in `api.yml`; CLAUDE.md, README, checklist, DNS, `.claude/commands`. *Done when:* PR checks green with DB tests running in CI.
7. **Verification.** Full walkthrough (15 rows) + site e2e parity + Worker preview e2e. *Done when:* all PASS, evidence in execution notes.

## Execution notes (2026-10-12)

All seven tasks done on `feature/dainq/goose-render-cloudflare-site-cutover` (PR #1).

- **Behaviour changes worth knowing:** inviting an existing email now returns 409 `EMAIL_TAKEN` (was: silently reuse the Supabase account); invite requires an initial password (10–72 chars); SVG uploads are rejected; refresh re-reads the account so a deleted user's session ends at the next refresh (access tokens live ≤ 15 min).
- **Deviation:** goose's version table is not guarded by an advisory lock in this repo's `migrate` package (single Render instance), so `MIGRATIONS_DATABASE_URL` exists to keep DDL off the pooler, not for lock safety.
- **Found and fixed while verifying:** `apps/web/.env.example` was never committed (ignored by `.env*`); `rbac` store test leaked one user per run (pool closed before `t.Cleanup`).
- **Evidence:** API `go test -race ./...` with DB tests (local Postgres 18 and CI, 0 skipped), golangci-lint 0 issues; real-binary smoke (seed, login, refresh misuse, upload/serve/delete, SVG rejected); admin Playwright 6/6 on `next dev` and 6/6 inside the OpenNext Worker preview; 15-row browser walkthrough all PASS; PR checks green.
- **Not verifiable locally:** real Neon and R2 endpoints. The R2 driver is tested against a fake S3 endpoint (request method, path-style key, content type); first contact with the real services happens on the Render deploy (see the checklist).
