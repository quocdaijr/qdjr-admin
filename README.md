# qdjr-admin

Admin dashboard backend (Go/Gin) and UI (Next.js) for [qdjr.me](https://qdjr.me), backed by Supabase. Replaces the legacy `cmsqdjr` Laravel CMS.

[![Plan 1](https://img.shields.io/badge/Plan%201-Foundation-success)](docs/superpowers/plans/2026-04-27-qdjr-admin-foundation.md)
[![Plan 2](https://img.shields.io/badge/Plan%202-Resources%20%2B%20UI-success)](docs/superpowers/plans/2026-04-27-qdjr-admin-resources.md)
[![Plan 3](https://img.shields.io/badge/Plan%203-Production-blue)](docs/superpowers/plans/2026-04-27-qdjr-admin-production.md)

---

## Overview

A small, modern, single-source-of-truth admin stack that costs $0 on free tiers.

```
┌──────────────┐        ┌────────────────────┐        ┌────────────────┐
│  qdjr (FE)   │◄──────►│  qdjr-admin-api    │◄──────►│   Supabase     │
│  Nuxt 3 SPA  │  REST  │  Go 1.23 + Gin     │  pgx   │   Postgres     │
│  qdjr.me     │        │  Cloud Run         │        │   + Auth       │
└──────────────┘        └────────────────────┘        │   + Storage    │
                              ▲                       └────────────────┘
                              │ Bearer JWT                   ▲
                              │                              │ Auth + signed URLs
                        ┌─────┴──────────┐                   │
                        │ qdjr-admin-web │───────────────────┘
                        │ Next.js 16     │
                        │ Vercel         │
                        │ admin.qdjr.me  │
                        └────────────────┘
```

## Tech stack

| Layer | Stack |
|---|---|
| **API** | Go 1.23+ · Gin v1.12 · `pgx/v5` · `golang-jwt/v5` · Supabase JWKS verification · Cloud Run (`asia-southeast1`) |
| **Web** | Next.js 16 (App Router · Turbopack) · React 19 · TypeScript · Tailwind 4 · shadcn/ui (base-ui) · TanStack Query/Table · react-hook-form + zod · Vercel |
| **Data** | Supabase Postgres 17 · Supabase Auth (email + JWKS) · Supabase Storage |
| **CI/CD** | GitHub Actions (api / web / migrations workflows) · Workload Identity Federation (no JSON keys) · Supabase CLI for forward-only migrations |

## Repo layout

```
qdjr-admin/
├── apps/
│   ├── api/                 # Go backend (Gin + pgx)
│   │   ├── cmd/api/         # main entrypoint
│   │   ├── cmd/bootstrap/   # one-shot CLI to seed first super_admin
│   │   └── internal/        # config, db, auth, rbac, http, posts, categories, tags, media, profile, sitesettings, contact, users, adminapi
│   └── web/                 # Next.js 16 admin UI
│       ├── app/             # App Router pages (login, admin/*)
│       ├── components/ui/   # shadcn primitives
│       ├── lib/             # API client, auth client, types
│       └── e2e/             # Playwright smoke tests
├── supabase/
│   ├── migrations/          # SQL — single source of truth for schema
│   └── seed.sql             # roles + singleton rows
├── infra/
│   ├── cloud-run/           # Cloud Run service config snapshot + capture script
│   └── dns/                 # DNS record reference
├── docs/superpowers/
│   ├── specs/               # design docs
│   ├── plans/               # implementation plans (1, 2, 3)
│   └── setup/               # one-time platform setup checklist
└── .github/workflows/       # api.yml · web.yml · migrations.yml
```

## Conventions

- **Response envelope** (always): `{ "data": ..., "meta": {...}, "error": null }` on success; `{ "data": null, "error": { "code": "...", "message": "..." } }` on failure. HTTP status set correctly.
- **Error codes**: SCREAMING_SNAKE_CASE (`UNAUTHENTICATED`, `FORBIDDEN`, `NOT_FOUND`, `VALIDATION`, `RATE_LIMITED`, `INTERNAL`, …).
- **Middleware order**: `RequireAuth → RequirePermission(...) → RequireOwnership(...)` (last only for the `author` role on resource-scoped routes).
- **Migrations are forward-only.** Rollback = a new migration. Never edit a committed migration.
- **Slugs**: lowercase, hyphenated, ASCII only, max 200 chars.

## RBAC

Three fixed roles. Permissions are hardcoded in Go behind a `PermissionResolver` interface so a future move to a DB-backed store is a one-line swap.

| Permission | super_admin | editor | author |
|---|---|---|---|
| `posts:read:all` | ✓ | ✓ | own only |
| `posts:write` | ✓ | ✓ | own only |
| `posts:publish` | ✓ | ✓ | ✗ |
| `taxonomy:write` (categories, tags) | ✓ | ✓ | ✗ |
| `media:write` | ✓ | ✓ | own only |
| `profile:write`, `settings:write` | ✓ | ✓ | ✗ |
| `contact:read`, `contact:write` | ✓ | ✓ | ✗ |
| `users:manage` | ✓ | ✗ | ✗ |

## API surface (`/v1`)

**Public** (no auth, served to qdjr):

```
GET  /v1/posts?page=&perPage=&category=&tag=&q=
GET  /v1/posts/:slug
GET  /v1/categories
GET  /v1/categories/:slug/posts
GET  /v1/tags
GET  /v1/tags/:slug/posts
GET  /v1/profile
GET  /v1/site-settings
POST /v1/contact                        (rate-limited 5/hr/IP)
```

**Admin** (Bearer JWT + RBAC):

```
GET                  /v1/admin/me
GET   | POST         /v1/admin/posts
GET   | PATCH | DEL  /v1/admin/posts/:id
POST                 /v1/admin/posts/:id/{publish,unpublish}
GET   | POST         /v1/admin/categories            GET | PATCH | DEL  /v1/admin/categories/:id
GET   | POST         /v1/admin/tags                  GET | PATCH | DEL  /v1/admin/tags/:id
GET                  /v1/admin/media
POST                 /v1/admin/media/{signed-url}    POST  /v1/admin/media   DEL /v1/admin/media/:id
GET   | PATCH        /v1/admin/profile
GET   | PATCH        /v1/admin/site-settings
GET                  /v1/admin/contact-messages      PATCH /v1/admin/contact-messages/:id
GET   | POST         /v1/admin/users                 PATCH /v1/admin/users/:id/role  DEL /v1/admin/users/:id
```

**Ops:** `GET /healthz`, `GET /readyz`

## Local development

Prerequisites: Go 1.23+, Node 20+, pnpm 10+, Docker, [Supabase CLI](https://supabase.com/docs/guides/local-development/cli/getting-started).

```bash
# 1. Start the local Supabase stack (Postgres + Auth + Storage)
supabase start

# 2. Apply migrations + seed
supabase db reset

# 3. Bootstrap a super_admin (creates auth user + assigns role)
cd apps/api
cp .env.example .env
# Fill BOOTSTRAP_ADMIN_EMAIL, BOOTSTRAP_ADMIN_PASSWORD, and the values from `supabase status --output env`
go run ./cmd/bootstrap

# 4. Run the API on :8080
go run ./cmd/api

# 5. In another terminal, run the admin UI on :3000
cd apps/web
cp .env.example .env.local
pnpm install
pnpm dev
```

Open http://localhost:3000/login and sign in as the bootstrap user.

## Testing

```bash
# Backend — unit + integration (against local Supabase)
cd apps/api
TEST_DATABASE_URL="postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable" \
TEST_SUPABASE_URL=http://127.0.0.1:54321 \
TEST_SUPABASE_SERVICE_ROLE_KEY="<from .env>" \
go test -race -cover ./...

# Web — typecheck + build
cd apps/web
pnpm exec tsc --noEmit && pnpm build

# Web — Playwright E2E (requires API + Web dev server running)
pnpm exec playwright test
```

## Status

| Plan | Scope | Status |
|---|---|---|
| **Plan 1 — Foundation** | Schema + Go skeleton + auth + RBAC + `/v1/admin/me` + bootstrap CLI | ✅ shipped @ `v0.1.0-foundation` |
| **Plan 2 — Resources + UI** | All 8 BE resources + admin Web UI + Playwright e2e | ✅ shipped @ `v0.2.0-resources` |
| **Plan 3 — Production** | CI/CD + Cloud Run + Vercel + qdjr cutover | ⚙️ code complete; manual platform setup pending |

## Deploy

After completing `docs/superpowers/setup/2026-04-27-deployment-checklist.md`:

```bash
git push origin main          # triggers api.yml, web.yml, migrations.yml
```

- Tail Cloud Run: `gcloud run services logs read qdjr-admin-api --region=asia-southeast1 --limit 200`
- Manual deploy: `gcloud run deploy qdjr-admin-api --region asia-southeast1 --source apps/api`

Custom domains: `api.qdjr.me` (Cloud Run), `admin.qdjr.me` (Vercel). DNS records in [`infra/dns/README.md`](infra/dns/README.md).

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — context and conventions for AI-assisted development
- [`docs/superpowers/specs/`](docs/superpowers/specs) — design docs
- [`docs/superpowers/plans/`](docs/superpowers/plans) — implementation plans
- [`docs/superpowers/setup/`](docs/superpowers/setup) — deployment checklist

## License

Private. © Nguyen Quoc Dai.
