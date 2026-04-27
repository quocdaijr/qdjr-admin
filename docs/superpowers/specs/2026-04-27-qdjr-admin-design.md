# qdjr-admin — Supabase-backed admin BE + UI to replace legacy cmsqdjr

**Status:** approved design (2026-04-27)
**Owner:** quocdaijr
**Supabase project:** `rknqbtaybeqdzwwonlmg`

## Context

The legacy CMS at `/private/var/www/html/personal/cmsqdjr` (Laravel 12 + MySQL + Redis modular monolith) is dated and over-engineered for current needs: unused 2FA stubs, optional Elasticsearch, Spatie permission matrix, hierarchical taxonomy that the FE never uses, and pivot tables with metadata that no callers consume. Meanwhile the public frontend `/private/var/www/html/personal/qdjr` (Nuxt 3 SPA) has already decoupled from the old REST API and now reads Markdown via `@nuxt/content`; the old `/v1/posts` endpoints are dormant.

This project rebuilds the admin stack in three pieces:

1. A fresh admin **backend** in Go (Gin) backed by Supabase (Postgres + Auth + Storage), with RBAC.
2. An admin **UI** built in this same project (Next.js 15 + shadcn/ui).
3. A re-wire of the qdjr FE to consume the new BE. Existing `plugins/api.ts` shapes (lines 16-47) guide the public response contract so the FE diff stays small.

Migration from legacy MySQL is a **nice-to-have**, not a requirement. Outcome: a small, modern, single-source-of-truth admin stack that costs $0 on free tiers (Cloud Run + Vercel + Supabase free), with clean boundaries between auth, RBAC, persistence, and HTTP.

## High-level architecture

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
                        │ Next.js 15     │
                        │ Vercel         │
                        │ admin.qdjr.me  │
                        └────────────────┘
```

Three deployables, one Git monorepo at `/private/var/www/html/personal/qdjr-admin`:

- `apps/api/` — Go 1.23, Gin, `pgx/v5`, deployed to Cloud Run `asia-southeast1`
- `apps/web/` — Next.js 15 (App Router) + shadcn/ui + TanStack Query/Table + react-hook-form + zod, deployed to Vercel
- `supabase/` — migrations + seed, applied via Supabase CLI to project `rknqbtaybeqdzwwonlmg`

**Auth flow:** admin UI logs in via Supabase Auth (email+password) → JWT → sent to Go BE → BE verifies the JWT signature (HS256 with the Supabase JWT secret if the project is on the legacy key model, or RS256/ES256 via JWKS if on the new asymmetric key model — verifier supports both), loads the user's role from `user_roles`, enforces RBAC.

**Media flow:** UI requests a signed upload URL from BE → uploads file directly to Supabase Storage → POSTs metadata to BE to register a `media` record.

**Public flow:** qdjr FE calls `GET /v1/posts` etc. without auth; BE returns published-only data.

## Data model (Supabase Postgres)

Migrations live in `supabase/migrations/` and are the single source of truth. The Go BE does not own schema migrations.

**RBAC tables:**
- `roles(id smallint pk, name text unique, description text)` — seeded with `super_admin`, `editor`, `author`
- `user_roles(user_id uuid pk references auth.users on delete cascade, role_id smallint references roles, assigned_at timestamptz)`

Permissions are NOT a table initially — they live behind a Go interface (see RBAC section) so they can move to Postgres later without changing call sites.

**Content:**
- `posts(id uuid pk, slug text unique, title text, excerpt text, content text /*markdown*/, status post_status, thumbnail_id uuid → media, published_at timestamptz, meta_title text, meta_description text, og_image_id uuid → media, created_by uuid, updated_by uuid, created_at timestamptz, updated_at timestamptz)`
- `post_status` enum: `draft | published | archived`
- `categories(id uuid pk, slug text unique, name text, description text, created_at, updated_at)` — flat, no `parent_id`
- `tags(id uuid pk, slug text unique, name text, description text, created_at, updated_at)`
- `post_categories(post_id, category_id, primary key (post_id, category_id))`
- `post_tags(post_id, tag_id, primary key (post_id, tag_id))`
- `media(id uuid pk, filename text, storage_path text unique, mime_type text, size bigint, width int, height int, alt_text text, uploaded_by uuid references auth.users on delete set null, created_at)`

All `created_by` / `updated_by` / `uploaded_by` columns are `uuid references auth.users on delete set null`. Cascading delete is intentionally avoided — losing an admin user must not delete their content.

**Singletons** (single-row tables, enforced via `check (id = 1)`):
- `profile(id smallint pk default 1, full_name, bio, avatar_id uuid → media, tagline, social_links jsonb, location, email, updated_by, updated_at)`
- `site_settings(id smallint pk default 1, site_title, site_description, footer_text, contact_email, social_links jsonb, updated_by, updated_at)`

**Inbox:**
- `contact_messages(id uuid pk, name, email, subject, body, ip inet, user_agent, status contact_status, created_at)` — `contact_status`: `new | read | replied | spam`

**Indexes:**
- `posts(status, published_at desc)` — public listing
- `posts(slug)` unique
- `post_tags(tag_id)`, `post_categories(category_id)` — tag/category filters
- `categories(slug)`, `tags(slug)` unique
- `contact_messages(status, created_at desc)`

**RLS:** disabled on app tables. The BE uses the service-role connection and enforces auth itself. Only Supabase-managed `auth.*` tables retain default RLS.

## RBAC

Three fixed roles. Permissions hardcoded in Go behind an interface so a future move to a DB-backed permission store is a one-line swap:

```go
// apps/api/internal/rbac/resolver.go
type PermissionResolver interface {
    Can(ctx context.Context, userID uuid.UUID, perm string) (bool, error)
    Permissions(ctx context.Context, userID uuid.UUID) ([]string, error)
    Role(ctx context.Context, userID uuid.UUID) (string, error)
}
```

Initial implementation: `StaticPermissionResolver` with a `map[string][]string` (role → permissions) plus a lookup of the user's role from `user_roles`. Future `DBPermissionResolver` queries `permissions` + `role_permissions` — call sites do not change.

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

Middleware chain in `apps/api/internal/http/`:
- `RequireAuth()` — parses Bearer JWT, verifies signature (HS256 shared secret or asymmetric via JWKS), attaches `userID` + `role` to context
- `RequirePermission("posts:publish")` — calls `resolver.Can`
- `RequireOwnership(table)` — only triggered for the author role on resource-scoped routes; checks `created_by = userID`

**Bootstrap super_admin:** SQL seeds only insert role rows (the 3 fixed roles). The first super_admin is created by a one-shot Go CLI at `apps/api/cmd/bootstrap/main.go` that takes `BOOTSTRAP_ADMIN_EMAIL` (and optionally `BOOTSTRAP_ADMIN_PASSWORD`), calls the Supabase Admin API to create the auth user if absent, and inserts the `user_roles` row in a single transaction. Subsequent users are invited from the admin UI via `POST /v1/admin/users`, which uses the same Supabase Admin API path server-side (using the service-role key) and inserts the `user_roles` row.

## API surface — `/v1`

**Public (no auth, served to qdjr):**
```
GET  /v1/posts?page=&perPage=&category=&tag=&q=
GET  /v1/posts/:slug
GET  /v1/categories
GET  /v1/categories/:slug/posts
GET  /v1/tags
GET  /v1/tags/:slug/posts
GET  /v1/profile
GET  /v1/site-settings                   # public-safe fields only
POST /v1/contact                         # rate-limited 5/hr/IP
```

**Admin (Bearer JWT + RBAC):**
```
GET                  /v1/admin/me
GET   | POST         /v1/admin/posts
GET   | PATCH | DEL  /v1/admin/posts/:id
POST                 /v1/admin/posts/:id/publish
POST                 /v1/admin/posts/:id/unpublish
GET   | POST         /v1/admin/categories
GET   | PATCH | DEL  /v1/admin/categories/:id
GET   | POST         /v1/admin/tags
GET   | PATCH | DEL  /v1/admin/tags/:id
GET                  /v1/admin/media
POST                 /v1/admin/media/signed-url
POST                 /v1/admin/media
DEL                  /v1/admin/media/:id
GET   | PATCH        /v1/admin/profile
GET   | PATCH        /v1/admin/site-settings
GET                  /v1/admin/contact-messages
PATCH                /v1/admin/contact-messages/:id
GET   | POST         /v1/admin/users
PATCH                /v1/admin/users/:id/role
DEL                  /v1/admin/users/:id
```

**Ops:** `GET /healthz`, `GET /readyz`

**Response envelopes:**
- success list: `{ "data": [...], "meta": { "page", "perPage", "total" }, "error": null }`
- success single: `{ "data": {...}, "error": null }`
- error: `{ "data": null, "error": { "code": "FORBIDDEN", "message": "..." } }` with appropriate HTTP status

The public envelope mirrors what `/private/var/www/html/personal/qdjr/plugins/api.ts:57-115` already expects (paginated `posts` with `data` + `meta`), so the FE re-wire is minimal.

## Critical files to create

```
qdjr-admin/
├── CLAUDE.md
├── .claude/
│   ├── settings.json
│   └── commands/
│       ├── run-api.md   run-web.md   db-reset.md
│       ├── migration.md test-api.md  deploy-api.md
├── .mcp.json                                 # ✓ exists
├── apps/api/
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── auth/jwt.go                       # Supabase JWT verifier
│   │   ├── rbac/resolver.go rbac/static.go
│   │   ├── http/router.go http/middleware.go http/respond.go
│   │   ├── db/pool.go
│   │   └── posts/ categories/ tags/ media/ profile/ settings/ contact/ users/
│   │       └── handler.go service.go repository.go
│   ├── go.mod  Dockerfile  .env.example  .golangci.yml
├── apps/web/
│   ├── app/(auth)/login/page.tsx
│   ├── app/(admin)/layout.tsx
│   ├── app/(admin)/posts/page.tsx posts/[id]/page.tsx
│   ├── app/(admin)/categories/page.tsx tags/page.tsx
│   ├── app/(admin)/media/page.tsx profile/page.tsx
│   ├── app/(admin)/settings/page.tsx contact/page.tsx
│   ├── app/(admin)/users/page.tsx
│   ├── components/ui/                        # shadcn primitives
│   ├── lib/api.ts                            # typed fetch client → Go BE
│   ├── lib/auth.ts                           # Supabase Auth client
│   ├── package.json  tsconfig.json  .env.example
├── supabase/
│   ├── migrations/
│   │   ├── 0001_init_enums.sql
│   │   ├── 0002_rbac.sql
│   │   ├── 0003_content.sql
│   │   ├── 0004_singletons.sql
│   │   ├── 0005_contact.sql
│   │   └── 0006_indexes.sql
│   ├── seed.sql
│   └── config.toml
├── docs/superpowers/specs/2026-04-27-qdjr-admin-design.md   # this file
└── .github/workflows/api.yml web.yml migrations.yml
```

## Existing references to reuse

- **Public response shape** must match `/private/var/www/html/personal/qdjr/plugins/api.ts:16-47` (Post / Category / Tag interfaces). Re-using these field names minimizes the FE re-wire diff: `id, slug, title, content, excerpt, published_at, thumbnail, location, category, tags[], created_at, updated_at`.
- **Markdown rendering** on the FE already uses `ContentRenderer` (`/private/var/www/html/personal/qdjr/pages/blog/[slug].vue:75`). Storing post `content` as Markdown text matches this; FE re-wire adds a thin adapter (e.g., `markdown-it` or `@nuxtjs/mdc`).
- **Slug + status semantics** stay close to legacy `cmsqdjr/Modules/Post/Entities/Post.php:38-51` so a future MySQL → Postgres data import is straightforward.

## Claude Code project setup

`CLAUDE.md` (concise, high-signal):
- One-paragraph project summary
- Pinned tech (Go 1.23 + Gin + pgx, Next.js 15 + shadcn/ui, Supabase, Cloud Run, Vercel)
- Conventions: response envelope, error codes, middleware order, slug rules, forward-only migrations, immutable updates
- File layout reference
- Local dev cheatsheet
- Pointer to `docs/superpowers/specs/`

`.claude/settings.json` allowedTools (no prompts for safe ops):
- `Bash(go test:*) Bash(go build:*) Bash(go run:*) Bash(go mod:*) Bash(golangci-lint:*) Bash(gofmt:*)`
- `Bash(pnpm install) Bash(pnpm dev) Bash(pnpm build) Bash(pnpm lint) Bash(pnpm test:*)`
- `Bash(supabase db:*) Bash(supabase migration:*) Bash(supabase start) Bash(supabase stop) Bash(supabase status)`
- `Bash(git status) Bash(git diff:*) Bash(git log:*) Bash(git branch)` (read-only)
- Hooks: post-edit `gofmt -w` for `*.go`, `prettier -w` for `*.{ts,tsx,json,md}`

`.claude/commands/`: `/run-api`, `/run-web`, `/db-reset`, `/migration <name>`, `/test-api`, `/deploy-api`.

`.mcp.json` ✓ — Supabase MCP already wired to project `rknqbtaybeqdzwwonlmg`.

## CI/CD (GitHub Actions)

| Workflow | Trigger | Steps |
|---|---|---|
| `api.yml` | push to main, paths `apps/api/**` | `go test -race -cover ./...` → `golangci-lint run` → build container → push to Artifact Registry → `gcloud run deploy qdjr-admin-api --region asia-southeast1` |
| `web.yml` | PRs only | typecheck + lint + build (Vercel auto-deploys main from its own integration) |
| `migrations.yml` | push to main, paths `supabase/migrations/**` | `supabase db push --db-url $SUPABASE_DB_URL` (forward-only; rollback = new migration) |

**Secrets:**
- Cloud Run runtime via GCP Secret Manager: `DATABASE_URL` (Supabase pooled), `SUPABASE_JWT_SECRET`, `SUPABASE_SERVICE_ROLE_KEY`, `SUPABASE_URL`, `BOOTSTRAP_ADMIN_EMAIL`
- Vercel env: `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `NEXT_PUBLIC_API_URL`
- GitHub Actions: `GCP_WORKLOAD_IDENTITY_PROVIDER`, `GCP_SERVICE_ACCOUNT`, `SUPABASE_DB_URL`, `SUPABASE_ACCESS_TOKEN`

## Deployment topology

- **Cloud Run** `asia-southeast1`, CPU 1 / 256 MiB, max instances 10, min 0 (scale-to-zero)
- **Custom domains:** `api.qdjr.me` (Cloud Run domain mapping), `admin.qdjr.me` (Vercel)
- **CORS allowlist:** `https://qdjr.me`, `https://admin.qdjr.me`, plus `http://localhost:3000` and `http://localhost:3001` in non-prod
- **Cold start budget:** ~150 ms (Go static binary on distroless); upgrade to min=1 only if it ever matters

## Local dev

- `supabase start` — local Postgres, Auth, Storage on Docker
- `cd apps/api && go run ./cmd/api` — BE on `:8080`
- `cd apps/web && pnpm dev` — UI on `:3000`
- `.env.example` files in both apps; copy to `.env`

## Testing

- **Go BE:** unit tests per package + integration via `testcontainers-go` against ephemeral Postgres. Coverage gate ≥ 80% on `internal/auth`, `internal/rbac`, and all handler packages. AAA structure, `testify` assertions.
- **Web admin:** Vitest for utility/hooks; Playwright for two E2E flows — login, and create-draft-then-publish-post. No coverage gate on UI initially.
- **Migrations:** `supabase db diff` in CI to detect schema drift between code and DB.
- **Smoke after deploy:** GH Actions step curls `https://api.qdjr.me/healthz` post-deploy.

## Verification — end-to-end

1. `supabase start` → `supabase db reset` applies migrations + seed → `roles` has 3 rows.
2. Sign up bootstrap user via Supabase Auth (or insert directly); seed migration links them as super_admin in `user_roles`.
3. `go run ./cmd/api` (port 8080) + `pnpm --filter web dev` (port 3000).
4. Open `http://localhost:3000/login`, log in → JWT in cookie; `GET /v1/admin/me` returns role `super_admin`.
5. Create a category, a tag, upload an image, create a draft post referencing them, publish it.
6. `curl http://localhost:8080/v1/posts` returns the published post with category + tag joined; matches the contract in `qdjr/plugins/api.ts:16-47`.
7. `curl -X POST http://localhost:8080/v1/contact -d '...'` six times rapidly → sixth gets HTTP 429.
8. Re-wire qdjr `plugins/api.ts` to point at the local BE → `/blog` page renders posts from BE.
9. Ship: push to `main` → `migrations.yml` applies migrations → `api.yml` deploys Cloud Run → Vercel auto-deploys web → smoke `https://api.qdjr.me/healthz` and `https://admin.qdjr.me/login`.
10. Cut over qdjr FE prod env to `https://api.qdjr.me/v1` once parity is confirmed.

## Out of scope

- Data import from legacy MySQL `cmsqdjr` (deferred; user said optional). If pursued later: a one-off Go CLI under `apps/api/cmd/import-legacy/`.
- Comments, newsletter, search beyond `q` substring filter, analytics dashboards, multi-language.
- Custom admin theme branding beyond shadcn defaults.
- Stronger integrity for singleton tables beyond `check (id = 1)`.
