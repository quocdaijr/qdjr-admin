# qdjr-admin

Admin API (Go/Gin) and admin UI (Next.js) for [qdjr.dev](https://qdjr.dev), on Neon Postgres and Cloudflare R2. Replaces the legacy `cmsqdjr` Laravel CMS.

[![Plan 1](https://img.shields.io/badge/Plan%201-Foundation-success)](docs/superpowers/plans/2026-04-27-qdjr-admin-foundation.md)
[![Plan 2](https://img.shields.io/badge/Plan%202-Resources%20%2B%20UI-success)](docs/superpowers/plans/2026-04-27-qdjr-admin-resources.md)
[![Plan 3](https://img.shields.io/badge/Plan%203-Production-success)](docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md)

---

## Overview

A small, single-source-of-truth publishing stack that costs $0 on free tiers.

```
┌──────────────┐        ┌────────────────────┐        ┌────────────────┐
│  qdjr site   │◄──────►│  apps/api          │◄──────►│   Neon         │
│  Nuxt 4 SSR  │  REST  │  Go 1.27 + Gin     │  pgx   │   Postgres     │
│  Netlify     │        │  goose migrations  │        │   + Auth       │
│  qdjr.dev    │        │  Render (Docker)   │        │   + Storage    │
└──────────────┘        └────────────────────┘        └────────────────┘
                              ▲                              ▲
                              │ Bearer JWT                   │ Auth + signed URLs
                        ┌─────┴──────────┐                   │
                        │ apps/web       │───────────────────┘
                        │ Next.js 16     │
                        │ Cloudflare     │
                        │ *.workers.dev  │
                        └────────────────┘
```

## Tech stack

| Layer | Stack |
|---|---|
| **API** | Go 1.27 · Gin · `pgx/v5` · `golang-jwt/v5` · `pressly/goose` (embedded migrations) · own HS256 JWT + bcrypt auth · aws-sdk-go-v2 (R2) · Render (Docker, singapore) |
| **Web** | Next.js 16 (App Router · Turbopack) · React 19 · TypeScript · Tailwind 4 · shadcn/ui (base-ui) · TanStack Query · react-hook-form + zod · Cloudflare Workers (`@opennextjs/cloudflare`) · design system in `design.md` |
| **Site** | separate repo [`quocdaijr/qdjr`](https://github.com/quocdaijr/qdjr) (Nuxt 4, Netlify), reads `/v1/*` |
| **Data** | Neon Postgres 18 (pooled + direct URLs) · Cloudflare R2 for media (S3 API) |
| **CI/CD** | GitHub Actions (api / web / keepalive) · Render Blueprint · goose migrations applied by the API at startup |

## Repo layout

```
qdjr-admin/
├── apps/
│   ├── api/                 # Go backend (Gin + pgx)
│   │   ├── cmd/api/         # main entrypoint (runs migrations when RUN_MIGRATIONS=true)
│   │   ├── cmd/migrate/     # goose CLI: up · status · version · up-to N
│   │   ├── migrations/      # goose SQL migrations (embedded)
│   │   └── internal/        # config, db, migrate, auth, session, rbac, http, posts, categories, tags, media, profile, sitesettings, contact, users, adminapi
│   └── web/                 # Next.js 16 admin UI
│       ├── app/             # App Router pages (login, admin/*)
│       ├── components/      # page-header, status-chip, shadcn primitives in ui/
│       ├── lib/             # API client, auth client, types
│       └── e2e/             # Playwright smoke tests
├── docker-compose.yml       # local Postgres 18
├── infra/
│   └── dns/                 # DNS record reference
├── docs/superpowers/
│   ├── specs/               # design docs
│   ├── plans/               # implementation plans
│   └── setup/               # one-time platform setup checklist
├── design.md                # locked admin design system (Cobalt)
├── render.yaml              # Render Blueprint (API)
└── .github/workflows/       # api.yml · web.yml · keepalive.yml · backup.yml (nightly pg_dump → R2)
```

## Conventions

- **Response envelope** (always): `{ "data": ..., "meta": {...}, "error": null }` on success; `{ "data": null, "error": { "code": "...", "message": "..." } }` on failure. HTTP status set correctly.
- **Error codes**: SCREAMING_SNAKE_CASE (`UNAUTHENTICATED`, `FORBIDDEN`, `NOT_FOUND`, `VALIDATION`, `RATE_LIMITED`, `INTERNAL`, …).
- **Middleware order**: `RequireAuth → RequirePermission(...) → RequireOwnership(...)` (last only for the `author` role on resource-scoped routes).
- **Migrations are forward-only** (goose, `-- +goose Up` only). Rollback = a new migration. Never edit a committed migration.
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

**Auth** (public, rate-limited per IP):

```
POST /v1/auth/login     {email, password}  → {access_token, refresh_token, expires_in, user}
POST /v1/auth/refresh   {refresh_token}    → new pair
```

**Public** (no auth, served to qdjr.dev):

```
GET  /v1/posts?page=&perPage=&category=&tag=&q=
GET  /v1/posts/:slug
GET  /v1/categories
GET  /v1/categories/:slug/posts
GET  /v1/tags
GET  /v1/tags/:slug/posts
GET  /v1/profile                        (includes `content`: bilingual site copy)
GET  /v1/site-settings
POST /v1/contact                        (rate-limited 5/hr/IP)
```

**Admin** (Bearer JWT + RBAC):

```
GET                  /v1/admin/me
PATCH                /v1/admin/me/password           {current_password, new_password}
GET   | POST         /v1/admin/posts
GET   | PATCH | DEL  /v1/admin/posts/:id
POST                 /v1/admin/posts/:id/{publish,unpublish}
GET   | POST         /v1/admin/categories            GET | PATCH | DEL  /v1/admin/categories/:id
GET   | POST         /v1/admin/tags                  GET | PATCH | DEL  /v1/admin/tags/:id
GET                  /v1/admin/media
POST                 /v1/admin/media (multipart: file, alt_text)              DEL /v1/admin/media/:id
GET   | PATCH        /v1/admin/profile
GET   | PATCH        /v1/admin/site-settings
GET                  /v1/admin/contact-messages      PATCH /v1/admin/contact-messages/:id
GET   | POST         /v1/admin/users                 PATCH /v1/admin/users/:id/role  DEL /v1/admin/users/:id
```

**Ops:** `GET /healthz`, `GET /readyz`

## Local development

Prerequisites: Go 1.27+, Node 24+, pnpm 12 (pinned in apps/web), Docker.

```bash
# 1. Start local Postgres 18
docker compose up -d --wait db

# 2. Apply migrations (goose, embedded in the API)
cd apps/api
cp .env.example .env
# Set JWT_SECRET (openssl rand -hex 32); BOOTSTRAP_ADMIN_* seed the first super_admin
env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up

# 3. Run the API on :8080 (creates the bootstrap admin on first start;
#    media goes to tmp/media unless R2_* are set)
env $(grep -v '^#' .env | xargs) go run ./cmd/api

# 5. In another terminal, run the admin UI on :3000
cd apps/web
cp .env.example .env.local
pnpm install
pnpm dev

# The public site lives in quocdaijr/qdjr; run it there with
# API_URL=http://localhost:8080 npm run dev -- -p 3001
```

Open http://localhost:3000/login and sign in as the bootstrap user. Reset everything with `docker compose down -v db` then steps 1–3 again.

## Testing

```bash
# Backend — unit + integration (against the local Postgres; CI does the same)
cd apps/api
TEST_DATABASE_URL="postgres://qdjr:qdjr@127.0.0.1:5432/qdjr?sslmode=disable" go test -race -cover ./...

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
| **Plan 3 — Production** | goose + Render + Cloudflare + Netlify + site cutover + admin redesign + Neon/R2/own auth | ✅ code complete; see setup checklist |

## Deploy

After completing `docs/superpowers/setup/2026-10-12-deployment-checklist.md`:

```bash
git push origin main          # api.yml (tests), web.yml (deploy to Cloudflare); Render deploys from Git
```

- Migrations: embedded goose files in `apps/api/migrations`, applied by the API at startup (`RUN_MIGRATIONS=true` on Render). Locally `go run ./cmd/migrate up|status`.
- API logs: Render dashboard → `qdjr-admin-api` → Logs
- Manual admin deploy: `cd apps/web && pnpm run deploy`
- Backups: nightly `pg_dump` to the private R2 bucket `qdjr-backups` (newest 30). Restore: [`docs/runbooks/database-backup-restore.md`](docs/runbooks/database-backup-restore.md)

Domains: `api.qdjr.dev` (Render), `qdjr.dev` (Netlify), admin on `*.workers.dev`. DNS in [`infra/dns/README.md`](infra/dns/README.md).

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — context and conventions for AI-assisted development
- [`design.md`](design.md) — admin design system
- [`docs/superpowers/specs/`](docs/superpowers/specs) — design docs
- [`docs/superpowers/plans/`](docs/superpowers/plans) — implementation plans
- [`docs/superpowers/setup/`](docs/superpowers/setup) — deployment checklist

## License

Private. © Nguyen Quoc Dai.
