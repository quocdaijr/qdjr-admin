# qdjr-admin

Admin dashboard backend (Go/Gin) and UI (Next.js) for qdjr.me, backed by Supabase. Replaces legacy `cmsqdjr` (Laravel). Single-source-of-truth design lives in `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`.

## Tech (pinned)

- **API:** Go 1.23, Gin v1.10, `pgx/v5`, deployed to Google Cloud Run (`asia-southeast1`)
- **Web:** Next.js 15 (App Router) + shadcn/ui + TanStack Query/Table + react-hook-form + zod, deployed to Vercel
- **Data:** Supabase project `rknqbtaybeqdzwwonlmg` (Postgres + Auth + Storage)
- **Tooling:** Supabase CLI for migrations, GitHub Actions for CI

## Layout

```
apps/api/              # Go backend
apps/web/              # Next.js admin UI (Plan 2+)
supabase/migrations/   # SQL — single source of truth for schema
docs/superpowers/      # specs + plans
```

## Conventions

- **Response envelope (always):** `{ "data": ..., "meta": { ... }, "error": null }` for success; `{ "data": null, "error": { "code": "...", "message": "..." } }` for failure. HTTP status set correctly.
- **Error codes:** SCREAMING_SNAKE_CASE (`UNAUTHENTICATED`, `FORBIDDEN`, `NOT_FOUND`, `VALIDATION`, `RATE_LIMITED`, `INTERNAL`).
- **Middleware order:** `RequireAuth → RequirePermission(...) → RequireOwnership(...)` (last only for the `author` role on resource-scoped routes).
- **Migrations are forward-only.** Rollback = a new migration. Never edit a committed migration.
- **Slugs:** lowercase, hyphenated, ASCII only, max 200 chars.
- **Immutable updates in handlers:** never mutate input structs; build a new value to write.

## Local dev

```bash
supabase start                                              # local Postgres + Auth + Storage on Docker
cd apps/api && cp .env.example .env && go run ./cmd/api     # API on :8080
cd apps/web && cp .env.example .env && pnpm dev             # UI on :3000 (Plan 2+)
```

## See also

- Design spec: `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`
- Active plan: `docs/superpowers/plans/2026-04-27-qdjr-admin-foundation.md`
