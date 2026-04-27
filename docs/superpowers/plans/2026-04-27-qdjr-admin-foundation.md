# qdjr-admin Foundation — Implementation Plan (Plan 1 of 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the qdjr-admin monorepo with Claude Code config, the full Supabase schema, and a Go/Gin API skeleton that authenticates a Supabase JWT, looks up the user's role, enforces RBAC, and answers `GET /v1/admin/me` with the caller's role and permissions.

**Architecture:** Monorepo at `/private/var/www/html/personal/qdjr-admin`. Schema is owned by `supabase/migrations/*.sql` and applied with the Supabase CLI. The Go service in `apps/api/` is built around a small set of focused packages (`config`, `db`, `auth`, `rbac`, `http`, plus per-resource folders later). RBAC permissions live behind a `PermissionResolver` interface so a future DB-backed implementation is a one-line swap. Admin user bootstrap is a separate one-shot CLI.

**Tech Stack:** Go 1.23, Gin v1.10, `pgx/v5`, `github.com/golang-jwt/jwt/v5`, `github.com/MicahParks/keyfunc/v3` (JWKS), `github.com/google/uuid`, `github.com/stretchr/testify`, Supabase CLI, Docker (for `supabase start`).

**Spec reference:** `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`

**Out of scope for Plan 1:** all per-resource handlers (posts/categories/tags/etc.), the admin Web UI, CI/CD, deployment, qdjr FE re-wire. Those land in Plans 2 and 3.

**TDD discipline:** Every code task follows red-green-refactor: write failing test → run and confirm failure → minimal implementation → run and confirm pass → commit. Setup-only tasks (config files, CLAUDE.md, slash commands, migrations) skip the test step but still commit immediately.

---

## File structure

Files this plan creates or significantly touches:

```
qdjr-admin/
├── CLAUDE.md                                                 # Task 1
├── .claude/
│   ├── settings.json                                         # Task 2
│   └── commands/{run-api,run-web,db-reset,migration,test-api,deploy-api}.md   # Task 3
├── apps/api/
│   ├── go.mod  go.sum                                        # Task 12
│   ├── Dockerfile  .dockerignore                             # Task 13
│   ├── .env.example  .golangci.yml                           # Task 14
│   ├── cmd/api/main.go                                       # Task 22
│   ├── cmd/bootstrap/main.go                                 # Task 23
│   └── internal/
│       ├── config/config.go  config_test.go                  # Task 15
│       ├── db/pool.go  pool_test.go                          # Task 16
│       ├── http/respond.go  respond_test.go                  # Task 17
│       ├── http/router.go                                    # Task 18
│       ├── http/middleware.go  middleware_test.go            # Task 21
│       ├── auth/jwt.go  jwt_test.go                          # Task 19
│       ├── rbac/resolver.go                                  # Task 20
│       ├── rbac/static.go  static_test.go                    # Task 20
│       └── adminapi/me.go  me_test.go                        # Task 22
├── supabase/
│   ├── config.toml                                           # Task 4
│   ├── migrations/
│   │   ├── 0001_init_enums.sql                               # Task 5
│   │   ├── 0002_rbac.sql                                     # Task 6
│   │   ├── 0003_content.sql                                  # Task 7
│   │   ├── 0004_singletons.sql                               # Task 8
│   │   ├── 0005_contact.sql                                  # Task 9
│   │   └── 0006_indexes.sql                                  # Task 10
│   └── seed.sql                                              # Task 11
└── docs/superpowers/plans/2026-04-27-qdjr-admin-foundation.md  # this file
```

Each Go package has one clear responsibility:
- `config` — load env vars into a typed struct, fail fast on missing required values
- `db` — open and tune the pgx pool
- `auth` — verify Supabase JWTs (HS256 + JWKS), expose `Claims`
- `rbac` — `PermissionResolver` interface + static implementation
- `http` — Gin router wiring, middleware, response envelopes
- `adminapi` — admin endpoint handlers (only `me` in this plan; resources in Plan 2)

---

## Phase 0 — Claude Code project setup

### Task 1: Write CLAUDE.md

**Files:**
- Create: `CLAUDE.md`

- [ ] **Step 1: Create the file**

```markdown
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
supabase start                              # local Postgres + Auth + Storage on Docker
cd apps/api && cp .env.example .env && go run ./cmd/api    # API on :8080
cd apps/web && cp .env.example .env && pnpm dev            # UI on :3000 (Plan 2+)
```

## See also

- Design spec: `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`
- Active plan: `docs/superpowers/plans/2026-04-27-qdjr-admin-foundation.md`
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "chore: add CLAUDE.md with tech, conventions, and dev cheatsheet"
```

---

### Task 2: Write .claude/settings.json

**Files:**
- Create: `.claude/settings.json`

- [ ] **Step 1: Create the file**

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": [
      "Bash(go test:*)",
      "Bash(go build:*)",
      "Bash(go run:*)",
      "Bash(go mod:*)",
      "Bash(go vet:*)",
      "Bash(gofmt:*)",
      "Bash(golangci-lint:*)",
      "Bash(pnpm install)",
      "Bash(pnpm dev)",
      "Bash(pnpm build)",
      "Bash(pnpm lint)",
      "Bash(pnpm test:*)",
      "Bash(pnpm typecheck)",
      "Bash(supabase start)",
      "Bash(supabase stop)",
      "Bash(supabase status)",
      "Bash(supabase db:*)",
      "Bash(supabase migration:*)",
      "Bash(git status)",
      "Bash(git diff:*)",
      "Bash(git log:*)",
      "Bash(git branch)",
      "Bash(git show:*)"
    ]
  },
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "if echo \"$CLAUDE_FILE_PATHS\" | grep -qE '\\.go$'; then gofmt -w $CLAUDE_FILE_PATHS; fi"
          }
        ]
      }
    ]
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add .claude/settings.json
git commit -m "chore: add Claude Code project settings (allow-list + gofmt hook)"
```

---

### Task 3: Write project slash commands

**Files:**
- Create: `.claude/commands/run-api.md`
- Create: `.claude/commands/run-web.md`
- Create: `.claude/commands/db-reset.md`
- Create: `.claude/commands/migration.md`
- Create: `.claude/commands/test-api.md`
- Create: `.claude/commands/deploy-api.md`

- [ ] **Step 1: Create `run-api.md`**

```markdown
---
description: Start the Go API locally on :8080
---

Run the Go API from `apps/api/` against the local Supabase stack.

```bash
cd apps/api && go run ./cmd/api
```

Prereqs:
- `supabase start` is running
- `apps/api/.env` exists (copy from `.env.example`)
```

- [ ] **Step 2: Create `run-web.md`**

```markdown
---
description: Start the Next.js admin UI locally on :3000
---

```bash
cd apps/web && pnpm dev
```

Prereqs (Plan 2+): `apps/web/.env.local` exists.
```

- [ ] **Step 3: Create `db-reset.md`**

```markdown
---
description: Reset the local Supabase database, reapply all migrations and seed
---

```bash
supabase db reset
```

This drops the local DB, replays every file in `supabase/migrations/` in order, then runs `supabase/seed.sql`. Safe to run any time during development; never run against production.
```

- [ ] **Step 4: Create `migration.md`**

```markdown
---
description: Create a new Supabase migration file
argument-hint: <name>
---

Create a new timestamped migration in `supabase/migrations/`:

```bash
supabase migration new $1
```

After editing the SQL, apply with `supabase db reset` (local) or push via the `migrations.yml` GitHub Actions workflow (remote, Plan 3).
```

- [ ] **Step 5: Create `test-api.md`**

```markdown
---
description: Run Go tests with race detector and coverage
---

```bash
cd apps/api && go test -race -cover ./...
```

For HTML coverage: `go test -race -coverprofile=coverage.out ./... && go tool cover -html=coverage.out`.
```

- [ ] **Step 6: Create `deploy-api.md`**

```markdown
---
description: Deploy the API container to Cloud Run (Plan 3+)
---

This command becomes active in Plan 3 once `.github/workflows/api.yml` is in place.
For now, deployment is manual via `gcloud run deploy` per the spec.
```

- [ ] **Step 7: Commit**

```bash
git add .claude/commands/
git commit -m "chore: add project slash commands"
```

---

## Phase 1 — Supabase schema

### Task 4: Initialize Supabase project locally

**Files:**
- Create: `supabase/config.toml`
- Modify: `.gitignore` (add `supabase/.env` if not present — already added in bootstrap commit)

- [ ] **Step 1: Run `supabase init` from repo root**

```bash
supabase init
```

Expected output: `Generated config.toml` and a `supabase/` directory containing `config.toml` plus empty `seed.sql`.

- [ ] **Step 2: Edit `supabase/config.toml`**

Set the project name and pin the major Postgres version. Leave other defaults.

```toml
project_id = "qdjr-admin"

[db]
major_version = 15
```

- [ ] **Step 3: Verify the local stack starts**

```bash
supabase start
```

Expected: prints API URL (`http://127.0.0.1:54321`), DB URL, Studio URL, anon key, service-role key, and JWT secret. Note these — you'll paste them into `apps/api/.env` later.

- [ ] **Step 4: Stop the stack to keep ports free until needed**

```bash
supabase stop
```

- [ ] **Step 5: Commit**

```bash
git add supabase/
git commit -m "chore(supabase): init local project (postgres 15)"
```

---

### Task 5: Migration 0001 — enums

**Files:**
- Create: `supabase/migrations/0001_init_enums.sql`

- [ ] **Step 1: Create the migration**

```sql
-- supabase/migrations/0001_init_enums.sql
create type public.post_status as enum ('draft', 'published', 'archived');
create type public.contact_status as enum ('new', 'read', 'replied', 'spam');
```

- [ ] **Step 2: Apply locally and inspect**

```bash
supabase start
supabase db reset
```

Then verify:

```bash
psql "$(supabase status --output env | grep DB_URL | cut -d= -f2- | tr -d '"')" -c "select unnest(enum_range(null::public.post_status));"
```

Expected: rows `draft`, `published`, `archived`.

- [ ] **Step 3: Commit**

```bash
git add supabase/migrations/0001_init_enums.sql
git commit -m "feat(db): add post_status and contact_status enums"
```

---

### Task 6: Migration 0002 — RBAC tables

**Files:**
- Create: `supabase/migrations/0002_rbac.sql`

- [ ] **Step 1: Create the migration**

```sql
-- supabase/migrations/0002_rbac.sql
create table public.roles (
    id          smallint primary key generated always as identity,
    name        text not null unique,
    description text
);

create table public.user_roles (
    user_id     uuid primary key references auth.users(id) on delete cascade,
    role_id     smallint not null references public.roles(id),
    assigned_at timestamptz not null default now()
);

create index user_roles_role_id_idx on public.user_roles (role_id);
```

- [ ] **Step 2: Apply and verify**

```bash
supabase db reset
psql "$(supabase status --output env | grep DB_URL | cut -d= -f2- | tr -d '"')" \
  -c "\d public.roles" -c "\d public.user_roles"
```

Expected: both tables shown with the columns above.

- [ ] **Step 3: Commit**

```bash
git add supabase/migrations/0002_rbac.sql
git commit -m "feat(db): add roles and user_roles tables"
```

---

### Task 7: Migration 0003 — content tables

**Files:**
- Create: `supabase/migrations/0003_content.sql`

- [ ] **Step 1: Create the migration**

```sql
-- supabase/migrations/0003_content.sql
create table public.media (
    id            uuid primary key default gen_random_uuid(),
    filename      text not null,
    storage_path  text not null unique,
    mime_type     text not null,
    size          bigint not null check (size >= 0),
    width         int,
    height        int,
    alt_text      text,
    uploaded_by   uuid references auth.users(id) on delete set null,
    created_at    timestamptz not null default now()
);

create table public.posts (
    id                uuid primary key default gen_random_uuid(),
    slug              text not null unique check (length(slug) <= 200 and slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    title             text not null,
    excerpt           text,
    content           text not null default '',
    status            public.post_status not null default 'draft',
    thumbnail_id      uuid references public.media(id) on delete set null,
    og_image_id       uuid references public.media(id) on delete set null,
    meta_title        text,
    meta_description  text,
    published_at      timestamptz,
    created_by        uuid references auth.users(id) on delete set null,
    updated_by        uuid references auth.users(id) on delete set null,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);

create table public.categories (
    id          uuid primary key default gen_random_uuid(),
    slug        text not null unique check (length(slug) <= 200 and slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name        text not null,
    description text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table public.tags (
    id          uuid primary key default gen_random_uuid(),
    slug        text not null unique check (length(slug) <= 200 and slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name        text not null,
    description text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table public.post_categories (
    post_id     uuid not null references public.posts(id) on delete cascade,
    category_id uuid not null references public.categories(id) on delete cascade,
    primary key (post_id, category_id)
);

create table public.post_tags (
    post_id uuid not null references public.posts(id) on delete cascade,
    tag_id  uuid not null references public.tags(id) on delete cascade,
    primary key (post_id, tag_id)
);
```

- [ ] **Step 2: Apply and verify**

```bash
supabase db reset
psql "$(supabase status --output env | grep DB_URL | cut -d= -f2- | tr -d '"')" \
  -c "\dt public.*"
```

Expected output includes: `media`, `posts`, `categories`, `tags`, `post_categories`, `post_tags`, plus `roles`, `user_roles`.

- [ ] **Step 3: Commit**

```bash
git add supabase/migrations/0003_content.sql
git commit -m "feat(db): add posts, categories, tags, media, and pivots"
```

---

### Task 8: Migration 0004 — singleton tables (profile, site_settings)

**Files:**
- Create: `supabase/migrations/0004_singletons.sql`

- [ ] **Step 1: Create the migration**

```sql
-- supabase/migrations/0004_singletons.sql
create table public.profile (
    id            smallint primary key default 1 check (id = 1),
    full_name     text,
    bio           text,
    avatar_id     uuid references public.media(id) on delete set null,
    tagline       text,
    social_links  jsonb not null default '{}'::jsonb,
    location      text,
    email         text,
    updated_by    uuid references auth.users(id) on delete set null,
    updated_at    timestamptz not null default now()
);

create table public.site_settings (
    id                smallint primary key default 1 check (id = 1),
    site_title        text not null default 'qdjr.me',
    site_description  text,
    footer_text       text,
    contact_email     text,
    social_links      jsonb not null default '{}'::jsonb,
    updated_by        uuid references auth.users(id) on delete set null,
    updated_at        timestamptz not null default now()
);
```

- [ ] **Step 2: Apply and verify singleton behavior**

```bash
supabase db reset
DB="$(supabase status --output env | grep DB_URL | cut -d= -f2- | tr -d '"')"
psql "$DB" -c "insert into public.profile (id) values (1);"
psql "$DB" -c "insert into public.profile (id) values (2);" 2>&1 || echo "rejected as expected"
```

Expected: first insert succeeds; second insert rejected with `check constraint "profile_id_check"`.

- [ ] **Step 3: Commit**

```bash
git add supabase/migrations/0004_singletons.sql
git commit -m "feat(db): add profile and site_settings singleton tables"
```

---

### Task 9: Migration 0005 — contact_messages

**Files:**
- Create: `supabase/migrations/0005_contact.sql`

- [ ] **Step 1: Create the migration**

```sql
-- supabase/migrations/0005_contact.sql
create table public.contact_messages (
    id          uuid primary key default gen_random_uuid(),
    name        text not null,
    email       text not null,
    subject     text,
    body        text not null,
    ip          inet,
    user_agent  text,
    status      public.contact_status not null default 'new',
    created_at  timestamptz not null default now()
);
```

- [ ] **Step 2: Apply and verify**

```bash
supabase db reset
psql "$(supabase status --output env | grep DB_URL | cut -d= -f2- | tr -d '"')" \
  -c "\d public.contact_messages"
```

- [ ] **Step 3: Commit**

```bash
git add supabase/migrations/0005_contact.sql
git commit -m "feat(db): add contact_messages table"
```

---

### Task 10: Migration 0006 — indexes

**Files:**
- Create: `supabase/migrations/0006_indexes.sql`

- [ ] **Step 1: Create the migration**

```sql
-- supabase/migrations/0006_indexes.sql
create index posts_status_published_at_idx on public.posts (status, published_at desc);
create index posts_created_by_idx on public.posts (created_by);
create index post_tags_tag_id_idx on public.post_tags (tag_id);
create index post_categories_category_id_idx on public.post_categories (category_id);
create index contact_messages_status_created_at_idx on public.contact_messages (status, created_at desc);
create index media_uploaded_by_idx on public.media (uploaded_by);
```

- [ ] **Step 2: Apply and verify**

```bash
supabase db reset
psql "$(supabase status --output env | grep DB_URL | cut -d= -f2- | tr -d '"')" \
  -c "\di public.*"
```

Expected: list shows the six new indexes plus the implicit unique indexes from earlier migrations.

- [ ] **Step 3: Commit**

```bash
git add supabase/migrations/0006_indexes.sql
git commit -m "feat(db): add hot-path indexes"
```

---

### Task 11: Seed roles and singleton rows

**Files:**
- Modify: `supabase/seed.sql`

- [ ] **Step 1: Replace the file contents**

```sql
-- supabase/seed.sql
-- Idempotent seed: roles + default singleton rows.

insert into public.roles (name, description) values
    ('super_admin', 'Full system access including user management'),
    ('editor',      'Manage all content; cannot manage users'),
    ('author',      'Create and edit own posts; cannot publish or manage taxonomy')
on conflict (name) do nothing;

insert into public.profile (id) values (1) on conflict (id) do nothing;
insert into public.site_settings (id) values (1) on conflict (id) do nothing;
```

- [ ] **Step 2: Apply and verify**

```bash
supabase db reset
DB="$(supabase status --output env | grep DB_URL | cut -d= -f2- | tr -d '"')"
psql "$DB" -c "select id, name from public.roles order by id;"
psql "$DB" -c "select id from public.profile;"
psql "$DB" -c "select id from public.site_settings;"
```

Expected: 3 roles, 1 row in each singleton.

- [ ] **Step 3: Commit**

```bash
git add supabase/seed.sql
git commit -m "feat(db): seed roles and singleton rows"
```

---

## Phase 2 — Go API skeleton

### Task 12: Initialize the Go module

**Files:**
- Create: `apps/api/go.mod`
- Create: `apps/api/go.sum`

- [ ] **Step 1: Create the module**

```bash
mkdir -p apps/api && cd apps/api && go mod init github.com/quocdaijr/qdjr-admin/apps/api
```

- [ ] **Step 2: Pin Go 1.23 and add core deps**

```bash
cd apps/api
go get github.com/gin-gonic/gin@v1.10.0
go get github.com/jackc/pgx/v5@v5.6.0
go get github.com/jackc/pgx/v5/pgxpool@v5.6.0
go get github.com/golang-jwt/jwt/v5@v5.2.1
go get github.com/MicahParks/keyfunc/v3@v3.3.5
go get github.com/google/uuid@v1.6.0
go get github.com/stretchr/testify@v1.9.0
```

- [ ] **Step 3: Tidy and verify**

```bash
cd apps/api && go mod tidy && cat go.mod
```

Expected: `go.mod` lists `go 1.23.X` and the required dependencies above.

- [ ] **Step 4: Commit**

```bash
git add apps/api/go.mod apps/api/go.sum
git commit -m "chore(api): initialize go module and pin core deps"
```

---

### Task 13: Add Dockerfile

**Files:**
- Create: `apps/api/Dockerfile`
- Create: `apps/api/.dockerignore`

- [ ] **Step 1: Create the Dockerfile**

```dockerfile
# apps/api/Dockerfile
# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /app/api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
```

- [ ] **Step 2: Create `.dockerignore`**

```
.git
.github
.idea
.vscode
**/*.md
**/testdata
**/tmp
**/*.test
coverage.out
```

- [ ] **Step 3: Verify it builds (will rebuild after main.go lands)**

For now, just confirm the file is valid. Don't build yet — `cmd/api` doesn't exist.

- [ ] **Step 4: Commit**

```bash
git add apps/api/Dockerfile apps/api/.dockerignore
git commit -m "chore(api): add multi-stage distroless Dockerfile"
```

---

### Task 14: Add `.env.example` and `.golangci.yml`

**Files:**
- Create: `apps/api/.env.example`
- Create: `apps/api/.golangci.yml`

- [ ] **Step 1: Create `.env.example`**

```dotenv
# apps/api/.env.example
# Copy to .env and fill in values from `supabase status` (local) or your secret manager (prod).

# HTTP
PORT=8080
ENV=development

# Database (Supabase pooled connection string)
DATABASE_URL=postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable

# Supabase
SUPABASE_URL=http://127.0.0.1:54321
SUPABASE_JWT_SECRET=super-secret-jwt-token-with-at-least-32-characters-long
SUPABASE_SERVICE_ROLE_KEY=
# Leave empty to use HS256 with SUPABASE_JWT_SECRET; set to enable JWKS verification.
SUPABASE_JWKS_URL=

# CORS allowlist (comma-separated)
CORS_ORIGINS=http://localhost:3000,http://localhost:3001

# Bootstrap CLI (apps/api/cmd/bootstrap)
BOOTSTRAP_ADMIN_EMAIL=
BOOTSTRAP_ADMIN_PASSWORD=
```

- [ ] **Step 2: Create `.golangci.yml`**

```yaml
# apps/api/.golangci.yml
run:
  timeout: 3m
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - revive
issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

- [ ] **Step 3: Commit**

```bash
git add apps/api/.env.example apps/api/.golangci.yml
git commit -m "chore(api): add env example and golangci-lint config"
```

---

### Task 15: Config loader

**Files:**
- Create: `apps/api/internal/config/config.go`
- Test: `apps/api/internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/config/config_test.go
package config

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoad_DefaultsAndRequired(t *testing.T) {
    t.Run("returns error when DATABASE_URL is missing", func(t *testing.T) {
        env := map[string]string{
            "SUPABASE_URL":        "http://localhost:54321",
            "SUPABASE_JWT_SECRET": "x",
        }
        _, err := Load(envGetter(env))
        require.Error(t, err)
        assert.Contains(t, err.Error(), "DATABASE_URL")
    })

    t.Run("loads defaults and parses CORS list", func(t *testing.T) {
        env := map[string]string{
            "DATABASE_URL":        "postgres://x",
            "SUPABASE_URL":        "http://localhost:54321",
            "SUPABASE_JWT_SECRET": "secret-secret-secret-secret-secret",
            "CORS_ORIGINS":        "http://a, http://b",
        }
        c, err := Load(envGetter(env))
        require.NoError(t, err)
        assert.Equal(t, "8080", c.Port)
        assert.Equal(t, "development", c.Env)
        assert.Equal(t, []string{"http://a", "http://b"}, c.CORSOrigins)
        assert.Equal(t, "secret-secret-secret-secret-secret", c.SupabaseJWTSecret)
    })
}

func envGetter(m map[string]string) func(string) string {
    return func(k string) string { return m[k] }
}
```

- [ ] **Step 2: Run the test — it must fail**

```bash
cd apps/api && go test ./internal/config/...
```

Expected: compile error — `undefined: Load`.

- [ ] **Step 3: Implement the loader**

```go
// apps/api/internal/config/config.go
package config

import (
    "fmt"
    "os"
    "strings"
)

// Config holds runtime configuration. All fields are populated from env vars.
type Config struct {
    Port                   string
    Env                    string
    DatabaseURL            string
    SupabaseURL            string
    SupabaseJWTSecret      string
    SupabaseJWKSURL        string
    SupabaseServiceRoleKey string
    CORSOrigins            []string
}

// Load reads configuration from the supplied env getter (use os.Getenv in main).
// Returns an error if a required value is missing.
func Load(get func(string) string) (Config, error) {
    c := Config{
        Port:                   getOr(get, "PORT", "8080"),
        Env:                    getOr(get, "ENV", "development"),
        DatabaseURL:            get("DATABASE_URL"),
        SupabaseURL:            get("SUPABASE_URL"),
        SupabaseJWTSecret:      get("SUPABASE_JWT_SECRET"),
        SupabaseJWKSURL:        get("SUPABASE_JWKS_URL"),
        SupabaseServiceRoleKey: get("SUPABASE_SERVICE_ROLE_KEY"),
        CORSOrigins:            parseCSV(get("CORS_ORIGINS")),
    }
    var missing []string
    if c.DatabaseURL == "" {
        missing = append(missing, "DATABASE_URL")
    }
    if c.SupabaseURL == "" {
        missing = append(missing, "SUPABASE_URL")
    }
    if c.SupabaseJWTSecret == "" && c.SupabaseJWKSURL == "" {
        missing = append(missing, "SUPABASE_JWT_SECRET or SUPABASE_JWKS_URL")
    }
    if len(missing) > 0 {
        return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
    }
    return c, nil
}

// LoadFromOS is a convenience wrapper for main().
func LoadFromOS() (Config, error) { return Load(os.Getenv) }

func getOr(get func(string) string, key, fallback string) string {
    if v := get(key); v != "" {
        return v
    }
    return fallback
}

func parseCSV(v string) []string {
    if v == "" {
        return nil
    }
    parts := strings.Split(v, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p != "" {
            out = append(out, p)
        }
    }
    return out
}
```

- [ ] **Step 4: Run the test — it must pass**

```bash
cd apps/api && go test ./internal/config/... -v
```

Expected: both subtests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/config/
git commit -m "feat(api/config): add typed config loader with required-field validation"
```

---

### Task 16: pgx pool wiring

**Files:**
- Create: `apps/api/internal/db/pool.go`
- Test: `apps/api/internal/db/pool_test.go`

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/db/pool_test.go
package db

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Skipped unless TEST_DATABASE_URL points at a running Postgres (use the local Supabase stack).
func TestNewPool_PingsDatabase(t *testing.T) {
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("set TEST_DATABASE_URL to run integration test")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    pool, err := NewPool(ctx, dsn)
    require.NoError(t, err)
    defer pool.Close()

    var one int
    err = pool.QueryRow(ctx, "select 1").Scan(&one)
    require.NoError(t, err)
    assert.Equal(t, 1, one)
}
```

- [ ] **Step 2: Run the test — it must fail to compile**

```bash
cd apps/api && go test ./internal/db/...
```

Expected: `undefined: NewPool`.

- [ ] **Step 3: Implement the pool**

```go
// apps/api/internal/db/pool.go
package db

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool and verifies connectivity with a Ping.
// Tunes pool size and lifetime for a typical Cloud Run workload (small + bursty).
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, fmt.Errorf("parse dsn: %w", err)
    }
    cfg.MaxConns = 10
    cfg.MinConns = 0
    cfg.MaxConnLifetime = 30 * time.Minute
    cfg.MaxConnIdleTime = 5 * time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, fmt.Errorf("open pool: %w", err)
    }
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, fmt.Errorf("ping: %w", err)
    }
    return pool, nil
}
```

- [ ] **Step 4: Run the integration test against local Supabase**

```bash
cd apps/api
supabase start                  # if not already running, from repo root
TEST_DATABASE_URL="postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable" \
  go test ./internal/db/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/db/
git commit -m "feat(api/db): add pgx pool with sane Cloud Run tuning"
```

---

### Task 17: Response envelope helpers

**Files:**
- Create: `apps/api/internal/http/respond.go`
- Test: `apps/api/internal/http/respond_test.go`

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/http/respond_test.go
package http

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestOK_WrapsDataAndNullsError(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/x", func(c *gin.Context) { OK(c, map[string]int{"n": 7}) })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

    require.Equal(t, http.StatusOK, rr.Code)
    var body struct {
        Data  map[string]int `json:"data"`
        Error any            `json:"error"`
    }
    require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
    assert.Equal(t, 7, body.Data["n"])
    assert.Nil(t, body.Error)
}

func TestList_IncludesMeta(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/x", func(c *gin.Context) { List(c, []int{1, 2}, Meta{Page: 1, PerPage: 10, Total: 2}) })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

    require.Equal(t, http.StatusOK, rr.Code)
    var body struct {
        Data []int `json:"data"`
        Meta Meta  `json:"meta"`
    }
    require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
    assert.Equal(t, []int{1, 2}, body.Data)
    assert.Equal(t, 2, body.Meta.Total)
}

func TestErr_SetsStatusAndCode(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/x", func(c *gin.Context) { Err(c, http.StatusForbidden, "FORBIDDEN", "no") })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

    require.Equal(t, http.StatusForbidden, rr.Code)
    var body struct {
        Data  any `json:"data"`
        Error struct {
            Code    string `json:"code"`
            Message string `json:"message"`
        } `json:"error"`
    }
    require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
    assert.Nil(t, body.Data)
    assert.Equal(t, "FORBIDDEN", body.Error.Code)
}
```

- [ ] **Step 2: Run the test — it must fail to compile**

```bash
cd apps/api && go test ./internal/http/...
```

Expected: `undefined: OK`, `undefined: List`, `undefined: Err`, `undefined: Meta`.

- [ ] **Step 3: Implement the helpers**

```go
// apps/api/internal/http/respond.go
package http

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

// Meta is the pagination envelope for list responses.
type Meta struct {
    Page    int `json:"page"`
    PerPage int `json:"perPage"`
    Total   int `json:"total"`
}

// errorBody mirrors the spec: { code, message }.
type errorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// OK writes a single-resource success response.
func OK(c *gin.Context, data any) {
    c.JSON(http.StatusOK, gin.H{"data": data, "error": nil})
}

// Created writes a 201 with the same envelope.
func Created(c *gin.Context, data any) {
    c.JSON(http.StatusCreated, gin.H{"data": data, "error": nil})
}

// NoContent writes a 204 (no envelope).
func NoContent(c *gin.Context) {
    c.Status(http.StatusNoContent)
}

// List writes a list success response with pagination meta.
func List(c *gin.Context, data any, meta Meta) {
    c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta, "error": nil})
}

// Err writes an error response with the given HTTP status, error code, and message.
func Err(c *gin.Context, status int, code, message string) {
    c.AbortWithStatusJSON(status, gin.H{
        "data":  nil,
        "error": errorBody{Code: code, Message: message},
    })
}
```

- [ ] **Step 4: Run the test — it must pass**

```bash
cd apps/api && go test ./internal/http/... -v
```

Expected: all three subtests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/http/respond.go apps/api/internal/http/respond_test.go
git commit -m "feat(api/http): add response envelope helpers (OK/List/Err)"
```

---

### Task 18: Router skeleton with health endpoints

**Files:**
- Create: `apps/api/internal/http/router.go`
- Modify: `apps/api/internal/http/respond_test.go` — no change; new tests live in router_test.go (next).
- Test: `apps/api/internal/http/router_test.go`

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/http/router_test.go
package http

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/assert"
)

func TestRouter_HealthEndpoints(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := NewRouter(RouterDeps{Pool: (*pgxpool.Pool)(nil), CORSOrigins: []string{"http://localhost:3000"}})

    for _, path := range []string{"/healthz", "/readyz"} {
        rr := httptest.NewRecorder()
        r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
        assert.Equal(t, http.StatusOK, rr.Code, "GET %s", path)
    }
}

func TestRouter_CORS_PreflightAllowsListedOrigin(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := NewRouter(RouterDeps{CORSOrigins: []string{"http://localhost:3000"}})

    req := httptest.NewRequest(http.MethodOptions, "/v1/posts", nil)
    req.Header.Set("Origin", "http://localhost:3000")
    req.Header.Set("Access-Control-Request-Method", "GET")
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)

    assert.Equal(t, "http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
}
```

- [ ] **Step 2: Run the test — it must fail to compile**

```bash
cd apps/api && go test ./internal/http/... -run Router
```

Expected: `undefined: NewRouter`, `undefined: RouterDeps`.

- [ ] **Step 3: Implement the router**

```go
// apps/api/internal/http/router.go
package http

import (
    "context"
    "net/http"
    "slices"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"
)

// RouterDeps wires runtime dependencies into the HTTP layer. New deps land here
// rather than as global state.
type RouterDeps struct {
    Pool        *pgxpool.Pool
    CORSOrigins []string
}

// NewRouter builds the Gin engine with the standard middleware stack and
// registers /healthz and /readyz. Resource routes are added by Plan 2.
func NewRouter(deps RouterDeps) *gin.Engine {
    r := gin.New()
    r.Use(gin.Recovery(), corsMiddleware(deps.CORSOrigins))

    r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
    r.GET("/readyz", func(c *gin.Context) {
        if deps.Pool == nil {
            c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "skipped"})
            return
        }
        ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
        defer cancel()
        if err := deps.Pool.Ping(ctx); err != nil {
            Err(c, http.StatusServiceUnavailable, "DB_UNREACHABLE", err.Error())
            return
        }
        c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "ok"})
    })

    return r
}

func corsMiddleware(allowed []string) gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        if origin != "" && slices.Contains(allowed, origin) {
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Vary", "Origin")
            c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
            c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
            c.Header("Access-Control-Max-Age", "600")
        }
        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        c.Next()
    }
}
```

- [ ] **Step 4: Run the test — it must pass**

```bash
cd apps/api && go test ./internal/http/... -v
```

Expected: all router and respond tests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/http/router.go apps/api/internal/http/router_test.go
git commit -m "feat(api/http): add router with health endpoints and CORS allowlist"
```

---

## Phase 3 — Auth and RBAC

### Task 19: Supabase JWT verifier

**Files:**
- Create: `apps/api/internal/auth/jwt.go`
- Test: `apps/api/internal/auth/jwt_test.go`

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/auth/jwt_test.go
package auth

import (
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestVerifyHS256_ValidToken(t *testing.T) {
    secret := "test-secret-test-secret-test-secret"
    userID := uuid.New()
    tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub":   userID.String(),
        "email": "alice@example.com",
        "exp":   time.Now().Add(1 * time.Hour).Unix(),
        "iat":   time.Now().Unix(),
    })
    signed, err := tok.SignedString([]byte(secret))
    require.NoError(t, err)

    v := NewHS256Verifier([]byte(secret))
    claims, err := v.Verify(signed)
    require.NoError(t, err)
    assert.Equal(t, userID, claims.UserID)
    assert.Equal(t, "alice@example.com", claims.Email)
}

func TestVerifyHS256_RejectsExpired(t *testing.T) {
    secret := "test-secret-test-secret-test-secret"
    tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": uuid.New().String(),
        "exp": time.Now().Add(-1 * time.Minute).Unix(),
    })
    signed, _ := tok.SignedString([]byte(secret))

    v := NewHS256Verifier([]byte(secret))
    _, err := v.Verify(signed)
    assert.Error(t, err)
}

func TestVerifyHS256_RejectsBadSignature(t *testing.T) {
    tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": uuid.New().String(),
        "exp": time.Now().Add(1 * time.Hour).Unix(),
    })
    signed, _ := tok.SignedString([]byte("wrong-secret-wrong-secret-wrong"))

    v := NewHS256Verifier([]byte("right-secret-right-secret-right"))
    _, err := v.Verify(signed)
    assert.Error(t, err)
}
```

- [ ] **Step 2: Run the test — it must fail to compile**

```bash
cd apps/api && go test ./internal/auth/...
```

Expected: `undefined: NewHS256Verifier`, `undefined: Claims`.

- [ ] **Step 3: Implement the verifier**

```go
// apps/api/internal/auth/jwt.go
package auth

import (
    "errors"
    "fmt"

    "github.com/MicahParks/keyfunc/v3"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

// Claims is the subset of JWT claims this service consumes.
type Claims struct {
    UserID uuid.UUID
    Email  string
}

// Verifier turns a raw bearer token into Claims, or returns an error.
type Verifier interface {
    Verify(token string) (Claims, error)
}

// NewHS256Verifier verifies tokens issued by Supabase's legacy shared-secret model.
func NewHS256Verifier(secret []byte) Verifier {
    return &hsVerifier{secret: secret}
}

// NewJWKSVerifier verifies tokens issued under Supabase's asymmetric key model.
// Pass the project's JWKS URL (e.g., https://<project-ref>.supabase.co/auth/v1/.well-known/jwks.json).
func NewJWKSVerifier(jwksURL string) (Verifier, error) {
    k, err := keyfunc.NewDefault([]string{jwksURL})
    if err != nil {
        return nil, fmt.Errorf("init jwks: %w", err)
    }
    return &jwksVerifier{keyfunc: k.Keyfunc}, nil
}

type hsVerifier struct{ secret []byte }

func (v *hsVerifier) Verify(token string) (Claims, error) {
    return verifyWith(token, func(t *jwt.Token) (any, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return v.secret, nil
    })
}

type jwksVerifier struct{ keyfunc jwt.Keyfunc }

func (v *jwksVerifier) Verify(token string) (Claims, error) {
    return verifyWith(token, v.keyfunc)
}

func verifyWith(token string, kf jwt.Keyfunc) (Claims, error) {
    parsed, err := jwt.Parse(token, kf, jwt.WithValidMethods([]string{"HS256", "RS256", "ES256"}))
    if err != nil {
        return Claims{}, err
    }
    if !parsed.Valid {
        return Claims{}, errors.New("invalid token")
    }
    mc, ok := parsed.Claims.(jwt.MapClaims)
    if !ok {
        return Claims{}, errors.New("unexpected claims type")
    }
    sub, _ := mc["sub"].(string)
    uid, err := uuid.Parse(sub)
    if err != nil {
        return Claims{}, fmt.Errorf("invalid sub: %w", err)
    }
    email, _ := mc["email"].(string)
    return Claims{UserID: uid, Email: email}, nil
}
```

- [ ] **Step 4: Run the test — it must pass**

```bash
cd apps/api && go test ./internal/auth/... -v
```

Expected: all three subtests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/auth/
git commit -m "feat(api/auth): add Supabase JWT verifier (HS256 + JWKS)"
```

---

### Task 20: RBAC resolver — interface and static implementation

**Files:**
- Create: `apps/api/internal/rbac/resolver.go`
- Create: `apps/api/internal/rbac/static.go`
- Test: `apps/api/internal/rbac/static_test.go`

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/rbac/static_test.go
package rbac

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

type fakeRoleStore struct {
    roleByUser map[uuid.UUID]string
}

func (f *fakeRoleStore) RoleForUser(_ context.Context, u uuid.UUID) (string, error) {
    if r, ok := f.roleByUser[u]; ok {
        return r, nil
    }
    return "", ErrNoRole
}

func TestStatic_Can_SuperAdminAlwaysAllowed(t *testing.T) {
    u := uuid.New()
    r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "super_admin"}})
    ok, err := r.Can(context.Background(), u, "users:manage")
    require.NoError(t, err)
    assert.True(t, ok)
}

func TestStatic_Can_AuthorCannotPublish(t *testing.T) {
    u := uuid.New()
    r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "author"}})
    ok, err := r.Can(context.Background(), u, "posts:publish")
    require.NoError(t, err)
    assert.False(t, ok)
}

func TestStatic_Can_EditorCannotManageUsers(t *testing.T) {
    u := uuid.New()
    r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "editor"}})
    ok, err := r.Can(context.Background(), u, "users:manage")
    require.NoError(t, err)
    assert.False(t, ok)
}

func TestStatic_Permissions_AuthorList(t *testing.T) {
    u := uuid.New()
    r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "author"}})
    perms, err := r.Permissions(context.Background(), u)
    require.NoError(t, err)
    assert.Contains(t, perms, "posts:write")
    assert.NotContains(t, perms, "posts:publish")
}

func TestStatic_Role_Unknown(t *testing.T) {
    r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{}})
    _, err := r.Role(context.Background(), uuid.New())
    assert.ErrorIs(t, err, ErrNoRole)
}
```

- [ ] **Step 2: Run the test — it must fail to compile**

```bash
cd apps/api && go test ./internal/rbac/...
```

Expected: undefined `NewStatic`, `ErrNoRole`.

- [ ] **Step 3: Implement the interface**

```go
// apps/api/internal/rbac/resolver.go
package rbac

import (
    "context"
    "errors"

    "github.com/google/uuid"
)

// ErrNoRole indicates the caller has no row in user_roles.
var ErrNoRole = errors.New("no role assigned")

// PermissionResolver decides whether a user can perform a permission.
// Future implementations may swap a static map for a DB-backed store.
type PermissionResolver interface {
    Can(ctx context.Context, userID uuid.UUID, perm string) (bool, error)
    Permissions(ctx context.Context, userID uuid.UUID) ([]string, error)
    Role(ctx context.Context, userID uuid.UUID) (string, error)
}

// RoleStore returns the role name for a user (or ErrNoRole).
type RoleStore interface {
    RoleForUser(ctx context.Context, userID uuid.UUID) (string, error)
}
```

```go
// apps/api/internal/rbac/static.go
package rbac

import (
    "context"
    "slices"

    "github.com/google/uuid"
)

// rolePermissions is the spec's RBAC matrix collapsed to a map.
// Author "own only" cases are enforced separately by RequireOwnership middleware.
var rolePermissions = map[string][]string{
    "super_admin": {
        "posts:read:all", "posts:write", "posts:publish",
        "taxonomy:write",
        "media:write",
        "profile:write", "settings:write",
        "contact:read", "contact:write",
        "users:manage",
    },
    "editor": {
        "posts:read:all", "posts:write", "posts:publish",
        "taxonomy:write",
        "media:write",
        "profile:write", "settings:write",
        "contact:read", "contact:write",
    },
    "author": {
        "posts:write", // ownership enforced by middleware
        "media:write", // ownership enforced by middleware
    },
}

type staticResolver struct{ store RoleStore }

// NewStatic builds a resolver backed by the hardcoded matrix above.
func NewStatic(store RoleStore) PermissionResolver { return &staticResolver{store: store} }

func (r *staticResolver) Role(ctx context.Context, userID uuid.UUID) (string, error) {
    return r.store.RoleForUser(ctx, userID)
}

func (r *staticResolver) Permissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
    role, err := r.store.RoleForUser(ctx, userID)
    if err != nil {
        return nil, err
    }
    perms := rolePermissions[role]
    out := make([]string, len(perms))
    copy(out, perms)
    return out, nil
}

func (r *staticResolver) Can(ctx context.Context, userID uuid.UUID, perm string) (bool, error) {
    role, err := r.store.RoleForUser(ctx, userID)
    if err != nil {
        return false, err
    }
    return slices.Contains(rolePermissions[role], perm), nil
}
```

- [ ] **Step 4: Run the test — it must pass**

```bash
cd apps/api && go test ./internal/rbac/... -v
```

Expected: all five subtests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/rbac/
git commit -m "feat(api/rbac): add PermissionResolver interface and static impl"
```

---

### Task 21: Auth + RBAC middleware

**Files:**
- Create: `apps/api/internal/http/middleware.go`
- Test: `apps/api/internal/http/middleware_test.go`

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/http/middleware_test.go
package http

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
)

const testSecret = "test-secret-test-secret-test-secret"

func newTestJWT(t *testing.T, sub uuid.UUID) string {
    t.Helper()
    tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": sub.String(),
        "exp": time.Now().Add(1 * time.Hour).Unix(),
    })
    s, err := tok.SignedString([]byte(testSecret))
    require.NoError(t, err)
    return s
}

type stubResolver struct {
    role  string
    perms []string
    err   error
}

func (s *stubResolver) Role(_ context.Context, _ uuid.UUID) (string, error) {
    if s.err != nil {
        return "", s.err
    }
    return s.role, nil
}
func (s *stubResolver) Permissions(_ context.Context, _ uuid.UUID) ([]string, error) {
    return s.perms, s.err
}
func (s *stubResolver) Can(_ context.Context, _ uuid.UUID, perm string) (bool, error) {
    if s.err != nil {
        return false, s.err
    }
    for _, p := range s.perms {
        if p == perm {
            return true, nil
        }
    }
    return false, nil
}

func TestRequireAuth_RejectsMissingHeader(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    v := auth.NewHS256Verifier([]byte(testSecret))
    r.GET("/x", RequireAuth(v), func(c *gin.Context) { c.Status(http.StatusOK) })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
    assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAuth_AttachesUserID(t *testing.T) {
    gin.SetMode(gin.TestMode)
    uid := uuid.New()
    r := gin.New()
    v := auth.NewHS256Verifier([]byte(testSecret))
    r.GET("/x", RequireAuth(v), func(c *gin.Context) {
        got, ok := UserIDFromContext(c)
        require.True(t, ok)
        assert.Equal(t, uid, got)
        c.Status(http.StatusOK)
    })

    req := httptest.NewRequest(http.MethodGet, "/x", nil)
    req.Header.Set("Authorization", "Bearer "+newTestJWT(t, uid))
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequirePermission_AllowsAndDenies(t *testing.T) {
    gin.SetMode(gin.TestMode)
    uid := uuid.New()
    v := auth.NewHS256Verifier([]byte(testSecret))

    cases := []struct {
        name    string
        perms   []string
        wantSts int
    }{
        {"allowed", []string{"posts:write"}, http.StatusOK},
        {"denied", []string{}, http.StatusForbidden},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            res := &stubResolver{role: "author", perms: tc.perms}
            r := gin.New()
            r.GET("/x", RequireAuth(v), RequirePermission(res, "posts:write"), func(c *gin.Context) { c.Status(http.StatusOK) })

            req := httptest.NewRequest(http.MethodGet, "/x", nil)
            req.Header.Set("Authorization", "Bearer "+newTestJWT(t, uid))
            rr := httptest.NewRecorder()
            r.ServeHTTP(rr, req)
            assert.Equal(t, tc.wantSts, rr.Code)
        })
    }
}
```

- [ ] **Step 2: Run the test — it must fail to compile**

```bash
cd apps/api && go test ./internal/http/... -run Require
```

Expected: undefined `RequireAuth`, `RequirePermission`, `UserIDFromContext`.

- [ ] **Step 3: Implement the middleware**

```go
// apps/api/internal/http/middleware.go
package http

import (
    "context"
    "errors"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
    "github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

type ctxKey string

const (
    ctxUserID ctxKey = "uid"
    ctxRole   ctxKey = "role"
)

// RequireAuth parses the Bearer JWT, verifies it, and stores user id + email
// in the request context.
func RequireAuth(v auth.Verifier) gin.HandlerFunc {
    return func(c *gin.Context) {
        h := c.GetHeader("Authorization")
        if !strings.HasPrefix(h, "Bearer ") {
            Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "missing bearer token")
            return
        }
        claims, err := v.Verify(strings.TrimPrefix(h, "Bearer "))
        if err != nil {
            Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", err.Error())
            return
        }
        ctx := context.WithValue(c.Request.Context(), ctxUserID, claims.UserID)
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}

// RequirePermission denies the request unless the resolver says the user has perm.
// On a missing role (ErrNoRole) returns 403 (treat as no permissions).
func RequirePermission(res rbac.PermissionResolver, perm string) gin.HandlerFunc {
    return func(c *gin.Context) {
        uid, ok := UserIDFromContext(c)
        if !ok {
            Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
            return
        }
        ok, err := res.Can(c.Request.Context(), uid, perm)
        switch {
        case errors.Is(err, rbac.ErrNoRole):
            Err(c, http.StatusForbidden, "FORBIDDEN", "no role assigned")
            return
        case err != nil:
            Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
            return
        case !ok:
            Err(c, http.StatusForbidden, "FORBIDDEN", "missing permission: "+perm)
            return
        }
        c.Next()
    }
}

// UserIDFromContext returns the authenticated caller's id (set by RequireAuth).
func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
    v := c.Request.Context().Value(ctxUserID)
    id, ok := v.(uuid.UUID)
    return id, ok
}
```

- [ ] **Step 4: Run the test — it must pass**

```bash
cd apps/api && go test ./internal/http/... -v
```

Expected: all middleware tests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/http/middleware.go apps/api/internal/http/middleware_test.go
git commit -m "feat(api/http): add RequireAuth and RequirePermission middleware"
```

---

## Phase 4 — First admin endpoint, role store, and bootstrap CLI

### Task 22: Postgres-backed role store + `/v1/admin/me` + main wiring

**Files:**
- Create: `apps/api/internal/rbac/pgstore.go`
- Test: `apps/api/internal/rbac/pgstore_test.go`
- Create: `apps/api/internal/adminapi/me.go`
- Test: `apps/api/internal/adminapi/me_test.go`
- Create: `apps/api/cmd/api/main.go`
- Modify: `apps/api/internal/http/router.go` — add admin group wiring

- [ ] **Step 1: Write the failing test for the role store**

```go
// apps/api/internal/rbac/pgstore_test.go
package rbac

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestPGStore_RoleForUser(t *testing.T) {
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("set TEST_DATABASE_URL")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    pool, err := pgxpool.New(ctx, dsn)
    require.NoError(t, err)
    defer pool.Close()

    // Insert a fake auth.users row; in real life Supabase Auth owns this.
    uid := uuid.New()
    _, err = pool.Exec(ctx,
        `insert into auth.users (id, email) values ($1, $2)`,
        uid, "store-test@example.com")
    require.NoError(t, err)
    t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `delete from auth.users where id = $1`, uid) })

    _, err = pool.Exec(ctx,
        `insert into public.user_roles (user_id, role_id)
         select $1, id from public.roles where name = 'editor'`,
        uid)
    require.NoError(t, err)

    s := NewPGStore(pool)
    role, err := s.RoleForUser(ctx, uid)
    require.NoError(t, err)
    assert.Equal(t, "editor", role)

    _, err = s.RoleForUser(ctx, uuid.New())
    assert.ErrorIs(t, err, ErrNoRole)
}
```

- [ ] **Step 2: Run — it must fail to compile**

```bash
cd apps/api && go test ./internal/rbac/...
```

Expected: `undefined: NewPGStore`.

- [ ] **Step 3: Implement the role store**

```go
// apps/api/internal/rbac/pgstore.go
package rbac

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct{ pool *pgxpool.Pool }

// NewPGStore returns a RoleStore backed by Postgres (joins user_roles → roles).
func NewPGStore(p *pgxpool.Pool) RoleStore { return &pgStore{pool: p} }

func (s *pgStore) RoleForUser(ctx context.Context, u uuid.UUID) (string, error) {
    const q = `select r.name
                 from public.user_roles ur
                 join public.roles r on r.id = ur.role_id
                where ur.user_id = $1`
    var name string
    err := s.pool.QueryRow(ctx, q, u).Scan(&name)
    if errors.Is(err, pgx.ErrNoRows) {
        return "", ErrNoRole
    }
    return name, err
}
```

- [ ] **Step 4: Run — it must pass against local Supabase**

```bash
cd apps/api
TEST_DATABASE_URL="postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable" \
  go test ./internal/rbac/... -v
```

Expected: PASS.

- [ ] **Step 5: Write failing test for `/v1/admin/me`**

```go
// apps/api/internal/adminapi/me_test.go
package adminapi

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
    apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

const testSecret = "test-secret-test-secret-test-secret"

type stubResolver struct {
    role  string
    perms []string
}

func (s *stubResolver) Role(_ context.Context, _ uuid.UUID) (string, error) {
    return s.role, nil
}
func (s *stubResolver) Permissions(_ context.Context, _ uuid.UUID) ([]string, error) {
    return s.perms, nil
}
func (s *stubResolver) Can(_ context.Context, _ uuid.UUID, p string) (bool, error) {
    for _, x := range s.perms {
        if x == p {
            return true, nil
        }
    }
    return false, nil
}

func TestMe_ReturnsUserAndRole(t *testing.T) {
    gin.SetMode(gin.TestMode)
    uid := uuid.New()
    res := &stubResolver{role: "editor", perms: []string{"posts:write"}}
    v := auth.NewHS256Verifier([]byte(testSecret))

    r := gin.New()
    g := r.Group("/v1/admin", apphttp.RequireAuth(v))
    Register(g, res)

    tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub":   uid.String(),
        "email": "alice@example.com",
        "exp":   time.Now().Add(time.Hour).Unix(),
    })
    signed, _ := tok.SignedString([]byte(testSecret))

    req := httptest.NewRequest(http.MethodGet, "/v1/admin/me", nil)
    req.Header.Set("Authorization", "Bearer "+signed)
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)

    require.Equal(t, http.StatusOK, rr.Code)
    var body struct {
        Data struct {
            UserID      string   `json:"user_id"`
            Role        string   `json:"role"`
            Permissions []string `json:"permissions"`
        } `json:"data"`
    }
    require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
    assert.Equal(t, uid.String(), body.Data.UserID)
    assert.Equal(t, "editor", body.Data.Role)
    assert.Contains(t, body.Data.Permissions, "posts:write")
}
```

- [ ] **Step 6: Run — it must fail to compile**

```bash
cd apps/api && go test ./internal/adminapi/...
```

Expected: undefined `Register`.

- [ ] **Step 7: Implement the handler**

```go
// apps/api/internal/adminapi/me.go
package adminapi

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"

    apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
    "github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// Register attaches admin endpoints to the given group. Plan 1 only registers
// /me; Plan 2 expands this with per-resource handlers.
func Register(g *gin.RouterGroup, res rbac.PermissionResolver) {
    g.GET("/me", meHandler(res))
}

func meHandler(res rbac.PermissionResolver) gin.HandlerFunc {
    return func(c *gin.Context) {
        uid, ok := apphttp.UserIDFromContext(c)
        if !ok {
            apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
            return
        }
        role, err := res.Role(c.Request.Context(), uid)
        switch {
        case errors.Is(err, rbac.ErrNoRole):
            apphttp.Err(c, http.StatusForbidden, "FORBIDDEN", "no role assigned")
            return
        case err != nil:
            apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
            return
        }
        perms, err := res.Permissions(c.Request.Context(), uid)
        if err != nil {
            apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
            return
        }
        apphttp.OK(c, gin.H{
            "user_id":     uid.String(),
            "role":        role,
            "permissions": perms,
        })
    }
}
```

- [ ] **Step 8: Run — it must pass**

```bash
cd apps/api && go test ./internal/adminapi/... -v
```

Expected: PASS.

- [ ] **Step 9: Wire the admin group into the router**

Modify `apps/api/internal/http/router.go` — add an `AdminGroup` to `RouterDeps` so `cmd/api/main.go` can register handlers without an import cycle. Replace the entire file with:

```go
// apps/api/internal/http/router.go
package http

import (
    "context"
    "net/http"
    "slices"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
)

// RouterDeps wires runtime dependencies into the HTTP layer.
type RouterDeps struct {
    Pool        *pgxpool.Pool
    CORSOrigins []string
    Verifier    auth.Verifier
    // RegisterAdmin attaches admin handlers to the protected /v1/admin group.
    // Kept as a callback to avoid an import cycle between http and adminapi.
    RegisterAdmin func(*gin.RouterGroup)
    // RegisterPublic attaches unauthenticated /v1 handlers (used by Plan 2).
    RegisterPublic func(*gin.RouterGroup)
}

// NewRouter builds the Gin engine.
func NewRouter(deps RouterDeps) *gin.Engine {
    r := gin.New()
    r.Use(gin.Recovery(), corsMiddleware(deps.CORSOrigins))

    r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
    r.GET("/readyz", func(c *gin.Context) {
        if deps.Pool == nil {
            c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "skipped"})
            return
        }
        ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
        defer cancel()
        if err := deps.Pool.Ping(ctx); err != nil {
            Err(c, http.StatusServiceUnavailable, "DB_UNREACHABLE", err.Error())
            return
        }
        c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "ok"})
    })

    v1 := r.Group("/v1")
    if deps.RegisterPublic != nil {
        deps.RegisterPublic(v1)
    }
    if deps.RegisterAdmin != nil && deps.Verifier != nil {
        admin := v1.Group("/admin", RequireAuth(deps.Verifier))
        deps.RegisterAdmin(admin)
    }

    return r
}

func corsMiddleware(allowed []string) gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        if origin != "" && slices.Contains(allowed, origin) {
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Vary", "Origin")
            c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
            c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
            c.Header("Access-Control-Max-Age", "600")
        }
        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        c.Next()
    }
}
```

- [ ] **Step 10: Re-run http tests to confirm router still passes**

```bash
cd apps/api && go test ./internal/http/... -v
```

Expected: all router/respond/middleware tests pass.

- [ ] **Step 11: Implement `cmd/api/main.go`**

```go
// apps/api/cmd/api/main.go
package main

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/quocdaijr/qdjr-admin/apps/api/internal/adminapi"
    "github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
    "github.com/quocdaijr/qdjr-admin/apps/api/internal/config"
    "github.com/quocdaijr/qdjr-admin/apps/api/internal/db"
    apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
    "github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

func main() {
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    if err := run(log); err != nil {
        log.Error("fatal", "err", err)
        os.Exit(1)
    }
}

func run(log *slog.Logger) error {
    cfg, err := config.LoadFromOS()
    if err != nil {
        return err
    }
    log.Info("config loaded", "env", cfg.Env, "port", cfg.Port)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    pool, err := db.NewPool(ctx, cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("db: %w", err)
    }
    defer pool.Close()

    var verifier auth.Verifier
    switch {
    case cfg.SupabaseJWKSURL != "":
        verifier, err = auth.NewJWKSVerifier(cfg.SupabaseJWKSURL)
        if err != nil {
            return fmt.Errorf("jwks: %w", err)
        }
    default:
        verifier = auth.NewHS256Verifier([]byte(cfg.SupabaseJWTSecret))
    }

    resolver := rbac.NewStatic(rbac.NewPGStore(pool))

    if cfg.Env == "production" {
        gin.SetMode(gin.ReleaseMode)
    }

    r := apphttp.NewRouter(apphttp.RouterDeps{
        Pool:        pool,
        CORSOrigins: cfg.CORSOrigins,
        Verifier:    verifier,
        RegisterAdmin: func(g *gin.RouterGroup) {
            adminapi.Register(g, resolver)
        },
    })

    srv := &http.Server{
        Addr:              ":" + cfg.Port,
        Handler:           r,
        ReadHeaderTimeout: 5 * time.Second,
    }
    errCh := make(chan error, 1)
    go func() {
        log.Info("listening", "addr", srv.Addr)
        errCh <- srv.ListenAndServe()
    }()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    select {
    case err := <-errCh:
        return err
    case <-sigCh:
        log.Info("shutting down")
        sCtx, sCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer sCancel()
        return srv.Shutdown(sCtx)
    }
}
```

- [ ] **Step 12: Build the binary**

```bash
cd apps/api && go build -o /tmp/qdjr-api ./cmd/api && rm /tmp/qdjr-api
```

Expected: clean build (no errors).

- [ ] **Step 13: Run all tests**

```bash
cd apps/api && go test -race -cover ./...
```

Expected: all packages PASS, coverage reported.

- [ ] **Step 14: Commit**

```bash
git add apps/api/internal/rbac/pgstore.go apps/api/internal/rbac/pgstore_test.go \
        apps/api/internal/adminapi/ apps/api/internal/http/router.go \
        apps/api/cmd/api/main.go
git commit -m "feat(api): wire main, pg role store, and GET /v1/admin/me"
```

---

### Task 23: Bootstrap CLI for the first super_admin

**Files:**
- Create: `apps/api/cmd/bootstrap/main.go`
- Test: skipped (this is a one-shot CLI; verify by hand in Task 24)

- [ ] **Step 1: Implement the CLI**

```go
// apps/api/cmd/bootstrap/main.go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "os"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/quocdaijr/qdjr-admin/apps/api/internal/config"
)

// Bootstrap creates (or finds) a Supabase Auth user with the given email and
// records them as super_admin in public.user_roles. Idempotent.
func main() {
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    if err := run(log); err != nil {
        log.Error("fatal", "err", err)
        os.Exit(1)
    }
}

func run(log *slog.Logger) error {
    cfg, err := config.LoadFromOS()
    if err != nil {
        return err
    }
    email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
    if email == "" {
        return fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL is required")
    }
    if cfg.SupabaseServiceRoleKey == "" {
        return fmt.Errorf("SUPABASE_SERVICE_ROLE_KEY is required for bootstrap")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    uid, err := ensureAuthUser(ctx, cfg, email, os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
    if err != nil {
        return fmt.Errorf("ensure auth user: %w", err)
    }
    log.Info("auth user ready", "user_id", uid, "email", email)

    pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("db: %w", err)
    }
    defer pool.Close()

    const upsert = `
        insert into public.user_roles (user_id, role_id)
        select $1, id from public.roles where name = 'super_admin'
        on conflict (user_id) do update set role_id = excluded.role_id, assigned_at = now()`
    if _, err := pool.Exec(ctx, upsert, uid); err != nil {
        return fmt.Errorf("upsert role: %w", err)
    }
    log.Info("super_admin role assigned", "user_id", uid)
    return nil
}

// ensureAuthUser returns the existing auth.users id for `email`, or creates one.
func ensureAuthUser(ctx context.Context, cfg config.Config, email, password string) (uuid.UUID, error) {
    base := cfg.SupabaseURL + "/auth/v1/admin/users"

    // Try GET first (filter by email).
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"?email="+email, nil)
    req.Header.Set("apikey", cfg.SupabaseServiceRoleKey)
    req.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceRoleKey)

    res, err := http.DefaultClient.Do(req)
    if err != nil {
        return uuid.Nil, err
    }
    body, _ := io.ReadAll(res.Body)
    res.Body.Close()
    if res.StatusCode == http.StatusOK {
        var listed struct {
            Users []struct {
                ID    string `json:"id"`
                Email string `json:"email"`
            } `json:"users"`
        }
        if err := json.Unmarshal(body, &listed); err == nil {
            for _, u := range listed.Users {
                if u.Email == email {
                    return uuid.Parse(u.ID)
                }
            }
        }
    }

    // Otherwise create.
    if password == "" {
        return uuid.Nil, fmt.Errorf("user not found and BOOTSTRAP_ADMIN_PASSWORD not set")
    }
    payload, _ := json.Marshal(map[string]any{
        "email":         email,
        "password":      password,
        "email_confirm": true,
    })
    creq, _ := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(payload))
    creq.Header.Set("apikey", cfg.SupabaseServiceRoleKey)
    creq.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceRoleKey)
    creq.Header.Set("Content-Type", "application/json")
    cres, err := http.DefaultClient.Do(creq)
    if err != nil {
        return uuid.Nil, err
    }
    cbody, _ := io.ReadAll(cres.Body)
    cres.Body.Close()
    if cres.StatusCode >= 300 {
        return uuid.Nil, fmt.Errorf("create user: %d %s", cres.StatusCode, string(cbody))
    }
    var created struct {
        ID string `json:"id"`
    }
    if err := json.Unmarshal(cbody, &created); err != nil {
        return uuid.Nil, fmt.Errorf("decode create response: %w", err)
    }
    return uuid.Parse(created.ID)
}
```

- [ ] **Step 2: Build the CLI**

```bash
cd apps/api && go build -o /tmp/qdjr-bootstrap ./cmd/bootstrap && rm /tmp/qdjr-bootstrap
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add apps/api/cmd/bootstrap/
git commit -m "feat(api): add bootstrap CLI to create the first super_admin"
```

---

### Task 24: End-to-end verification

This task is verification only — no code changes. It exercises the full Plan 1 scope locally.

- [ ] **Step 1: Reset local Supabase**

```bash
supabase stop || true
supabase start
supabase db reset
```

Expected: stack starts, migrations apply, seed inserts 3 roles + singleton rows.

- [ ] **Step 2: Capture local secrets into `apps/api/.env`**

```bash
cd apps/api
cp .env.example .env
# Edit .env to paste values from `supabase status --output env` (DB_URL, JWT secret, service-role key, anon key, API URL).
```

- [ ] **Step 3: Set `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` in `.env`, then run the bootstrap CLI**

```bash
cd apps/api
export $(grep -v '^#' .env | xargs)
go run ./cmd/bootstrap
```

Expected log lines: `auth user ready` and `super_admin role assigned`.

- [ ] **Step 4: Confirm the role row in Postgres**

```bash
psql "$DATABASE_URL" -c \
  "select u.email, r.name from auth.users u
     join public.user_roles ur on ur.user_id = u.id
     join public.roles r on r.id = ur.role_id;"
```

Expected: one row showing `BOOTSTRAP_ADMIN_EMAIL` mapped to `super_admin`.

- [ ] **Step 5: Start the API**

```bash
cd apps/api && go run ./cmd/api
```

Expected: log `listening addr=:8080`.

- [ ] **Step 6: Hit health endpoints**

```bash
curl -fsSL http://localhost:8080/healthz
curl -fsSL http://localhost:8080/readyz
```

Expected: both return `{"status":"ok"}` (readyz also includes `"db":"ok"`).

- [ ] **Step 7: Get a JWT for the bootstrap user via Supabase Auth**

```bash
curl -fsSL "http://127.0.0.1:54321/auth/v1/token?grant_type=password" \
  -H "apikey: $(grep ANON_KEY .env | cut -d= -f2-)" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$BOOTSTRAP_ADMIN_EMAIL\",\"password\":\"$BOOTSTRAP_ADMIN_PASSWORD\"}" \
  | tee /tmp/login.json
JWT=$(jq -r .access_token /tmp/login.json)
echo "$JWT" | head -c 40 ; echo
```

Expected: a JWT prefixed by `eyJ`.

- [ ] **Step 8: Call `/v1/admin/me`**

```bash
curl -fsSL http://localhost:8080/v1/admin/me -H "Authorization: Bearer $JWT" | jq .
```

Expected:

```json
{
  "data": {
    "user_id": "...",
    "role": "super_admin",
    "permissions": ["posts:read:all", "posts:write", "posts:publish", "..."]
  },
  "error": null
}
```

- [ ] **Step 9: Confirm a request without a token is rejected**

```bash
curl -i http://localhost:8080/v1/admin/me | head -1
```

Expected: `HTTP/1.1 401 Unauthorized`.

- [ ] **Step 10: Tag the milestone**

```bash
git tag -a v0.1.0-foundation -m "Plan 1 foundation: schema + Go BE + auth + RBAC + /v1/admin/me"
git log --oneline | head
```

Expected: ~20 small commits and the new tag.

---

## Verification — Plan 1 acceptance

The plan is done when **all** of the following are true:

1. `go test -race -cover ./...` from `apps/api/` is green and coverage on `internal/auth`, `internal/rbac`, `internal/http`, `internal/adminapi` is ≥ 80%.
2. `supabase db reset` applies all six migrations cleanly and `select * from public.roles` returns 3 rows.
3. `go run ./cmd/bootstrap` is idempotent — running it twice does not create a duplicate role row.
4. `GET /healthz` and `GET /readyz` return 200; `/readyz` reports `db: ok`.
5. With a valid Supabase JWT, `GET /v1/admin/me` returns the caller's role and permissions inside the standard envelope.
6. With no/invalid token, the same endpoint returns 401 with the error envelope.
7. `git log --oneline` shows small, atomic commits — one per task or sub-task.
