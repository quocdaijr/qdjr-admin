# qdjr-admin: goose migrations, Render + Cloudflare hosting, `apps/site` cutover to the API

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move schema migrations into the Go API under goose, redesign the admin UI as one locked Cobalt design system (Hallmark), get qdjr-admin actually deployed (Go API on Render, Next admin on Cloudflare Workers), bring the public Nuxt site into this repo as `apps/site`, and make the site read posts, taxonomy, profile and social links from the API instead of its bundled markdown and TypeScript data.

**Architecture:** Migrations become `apps/api/migrations/*.sql` (goose `-- +goose Up` files, forward-only), embedded in the binary; the API applies them at startup when `RUN_MIGRATIONS=true` (Render) and `cmd/migrate` runs them by hand locally, so a deploy and its schema change are one atomic unit and no separate migrations workflow or extra secrets exist. The Go API already exposes the public read surface (`/v1/posts`, `/v1/categories`, `/v1/tags`, `/v1/profile`, `/v1/site-settings`); the only schema addition is `profile.content jsonb` holding the bilingual `ProfileContent` blob the site renders on `/` and `/about`, edited in the admin as raw JSON. The Nuxt site gets a thin `$api` plugin plus two adapters (`toBlogPost`, `pickProfileContent`) so page templates barely change; the bundled markdown/TS copy is seeded into Postgres once by a migration and kept in the site only as an offline fallback. Hosting: `render.yaml` (API, Docker), `@opennextjs/cloudflare` + a GitHub Actions deploy job (admin), `netlify.toml` (site, already live on Netlify at qdjr.dev). No GCP, no Vercel, no `supabase db push`.

**Tech Stack:** Go 1.25 / Gin / pgx / `pressly/goose/v3` (v3.29.0) · Next 16.4 + `@opennextjs/cloudflare` + wrangler 4 · Nuxt 4 + `markdown-it` · Render Blueprint · Netlify · GitHub Actions · Supabase CLI only for the local Postgres (`supabase start`).

**Spec:** This plan is the spec (decisions taken with the user on 2026-10-12, recorded in *Context* below). Design background: `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`, Plan 3 task 9: `docs/superpowers/plans/2026-04-27-qdjr-admin-production.md`.

**Anchors verified against:** `qdjr-admin` `main` @ `25073af` (clean tree), `qdjr` `origin/main` @ `4f37a44`. Re-run `git status` and `git log -1` before starting; if `main` moved, re-check every `file:line` below.

---

## Context

- Nothing from Plan 3 was ever deployed: no `gcloud`/`vercel` CLI on this Mac, zero GitHub secrets, zero workflow runs, and `qdjr.me` is NXDOMAIN. The domain is now **`qdjr.dev`**, bought on Hostinger, DNS on Netlify (NS1), and the Nuxt site already runs there. The user chose: **goose** for migrations (compared against golang-migrate and the Supabase CLI), **API → Render**, **admin → Cloudflare Workers (OpenNext) on a `workers.dev` URL for now** (custom domain would need the zone on Cloudflare), **site stays on Netlify**, **plain copy of `qdjr` into `apps/site`**, **everything incl. the full profile moves to the API**.
- Why goose over golang-migrate: one file per change with an optional Down section (matches the forward-only rule), transactional rollback on failure instead of golang-migrate's "dirty" state that needs `force`, per-file `NO TRANSACTION`, Go-code migrations available, `embed.FS` support, active v3 line.
- Why migrate at API startup instead of a CI job: Render free instances have no pre-deploy hook and `max-instances` is 1, so startup is the only place where the schema change and the code that needs it are guaranteed to land together. If a migration fails the new instance never passes `/healthz` and Render keeps the old one serving. `cmd/migrate` still exists for local use and emergencies.
- `@opennextjs/cloudflare@1.20.10` requires `next >=16.3.8`; `apps/web` is on `16.2.4` → bump to `16.4.0`.
- `apps/api/go.mod` says `go 1.25.0` but the Dockerfile and CI pin Go 1.23 → fix to 1.25 while touching deploy.
- FortiClient on this Mac intermittently resets browser traffic to `*.onrender.com` (vault note `forticlient-blocks-browser-traffic`). Giving the API the custom domain `api.qdjr.dev` (CNAME at Netlify DNS) avoids that; use `curl` to disambiguate if the admin UI "can't reach the API" from this machine.
- Render free web services sleep after 15 min idle; a 10-minute GitHub Actions keepalive ping keeps the API warm within the 750 free instance-hours/month (31 × 24 = 744).
- Supabase's direct DB host is IPv6-only; Render and GitHub runners need the **Session pooler** URL (port 5432, IPv4). Transaction pooling (6543) breaks pgx prepared statements, so never use it here.

## Architecture

**Principle.** One source of truth per concern. The database schema lives only in `apps/api/migrations` (goose, embedded). Site content lives only in Postgres and is served only by `/v1/*`. The admin's look lives only in `design.md` + the token block in `apps/web/app/globals.css`. Every other copy (markdown posts, `profile.*.ts`, `SocialLinks` hrefs, shadcn neutral tokens) is either deleted or demoted to an offline fallback.

**Data flow after this plan.**

```
 admin (Cloudflare Worker) ──PATCH /v1/admin/*──►  api (Render, Go) ──pgx──► Supabase Postgres
                                                     ▲   │  startup: goose up (embedded 0001…0009)
 site (Netlify, Nuxt SSR) ──GET /v1/posts|profile|site-settings──┘
        │  on API error: posts → [], profile → bundled copy, social links → bundled hrefs
```

**Why it works.**
- Schema and code ship in one artifact: Render builds the image, the binary migrates before it listens, `/healthz` gates the swap, so a bad migration never serves traffic.
- The site renders from the API on every request (SSR), so an admin edit shows on the next page load with no rebuild. Fallbacks keep `/`, `/about` and `/blog` up while the free API instance is cold or down.
- The admin is a static client app talking to the API with a Supabase JWT; OpenNext only has to serve it, no server data fetching moves to the Worker.
- The admin redesign touches only the visual layer (tokens, shell, page headers, two page layouts). No route, query key, mutation or e2e-visible label changes.

**What this plan deliberately does not change:** API routes and response shapes (except the additive `profile.content`), RBAC, auth flow, the site's trips planner and 3D scenes, the media upload flow. See *Deliberately not done* at the end.

## Agent Execution Protocol

- **Order.** 0 → 1 → 2 → 3 → 4 → 5 → 5A → 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13. Hard dependencies: 2 needs 1; 4 needs 1–3; 5 needs 2; 5A needs 5 (it restyles the JSON editor); 6–8 need 3 and a running API from 4; 9–11 are independent of 5A–8 but must land before 12.
- **Dispatch prompt (one fresh implementer per task):**
  > You are implementing Task N of `docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md` in `/private/var/www/html/personal/qdjr-admin` on branch `feature/dainq/goose-render-cloudflare-site-cutover`. Read the plan header (Context, Architecture, Global Constraints, Review Focus) and Task N only. Apply every step exactly; FIND blocks must match byte-for-byte; if one does not, stop and report the actual text instead of improvising. Run every command and compare with its Expected line. Commit once at the end of the task with the given message. Report: files changed, commands run with their real output tails, and each "Done when" item ticked or failed.
- **Reviewer gate.** After each task a fresh reviewer reads the task's diff (`git show HEAD`) against the task text and Global Constraints, re-runs the task's verification commands, and answers PASS or a list of defects. Defects go back to the same implementer; the next task starts only on PASS.
- **Failure handling.** A failing Expected line is a stop, not a retry loop: implementer reports the output; if the cause is environmental (Docker down, port taken, FortiClient reset on `*.onrender.com`), fix the environment and rerun; if the plan text is wrong, the controller amends the plan file first, then the implementer reruns.
- **Final gate.** After Task 13, one reviewer on the most capable model reviews `git diff main...HEAD` end to end.

## Global Constraints

- Response envelope and error codes exactly as `CLAUDE.md` (`{data, meta, error}`; `VALIDATION`/`INVALID`, `NOT_FOUND`, …). Admin handlers already use `INVALID` for bad bodies; keep that.
- Migrations are forward-only, sequential `00NN_<name>.sql`, first line after the path comment is `-- +goose Up`, no `-- +goose Down` sections. Never edit an applied migration; rollback = a new migration. Statements containing `;` inside strings or dollar-quotes go between `-- +goose StatementBegin` / `-- +goose StatementEnd`.
- Handlers never mutate input structs; build new values (the profile `UpdateInput` pattern).
- Immutability in TS: build new objects, no in-place mutation.
- Slugs: lowercase, hyphenated ASCII, ≤ 200 chars (DB check constraint enforces).
- Commit format `<type>: <description>`, one logical change per commit, Co-Authored-By trailer as the session reminder says.
- No secrets in the repo: Supabase anon/publishable key and project URL are public and may be committed; `DATABASE_URL`, JWT secret, service-role key, Cloudflare token never.
- Never put the admin's URL or env values into the public site's DOM or console (vault pref `no-internal-urls-on-public-surfaces`).

## Review Focus

1. **API unreachable when the site renders** → `/`, `/about`, `/blog` must still render (static profile fallback, empty post list), not 500. Pinned in Task 6 (`pickProfileContent` null path) and Task 7 (`fetchBlogPosts` returns `[]` on error).
2. **Admin sends non-object `content`** (`[]`, `"x"`, `null`) → 400 `INVALID`, row untouched. Pinned in Task 2 handler tests.
3. **Post without category or thumbnail** → list and detail render without crashing. Pinned in Task 7 `toBlogPost` test.
4. **English locale** → every post shows the "Vietnamese only" badge/notice (posts are single-language in the DB). Pinned in Task 7 `toBlogPost` test and existing e2e `blog-i18n.spec.ts`.
5. **A typo'd `cmd/migrate down` or a migration with a gap in numbering** must be refused before it touches a database. Pinned in Task 1 (`Run` rejects non-allowlisted commands; `TestEmbeddedMigrationsAreContiguous`). Re-running `up` on an already-migrated DB is a no-op (goose version table) — verified in Task 1 step 7 and Task 4 step 3.

## File structure

| Path | Responsibility |
|---|---|
| `apps/api/migrations/00NN_*.sql`, `apps/api/migrations/embed.go` | goose SQL migrations, embedded (`migrations.FS`) |
| `apps/api/internal/migrate/migrate.go` | `Run(ctx, dsn, cmd, args…)`, `Up(ctx, dsn)` over goose; allowlisted commands |
| `apps/api/cmd/migrate/main.go` | CLI: `go run ./cmd/migrate [up\|status\|version\|up-by-one\|up-to N]` |
| `apps/api/internal/config/config.go`, `apps/api/cmd/api/main.go` | `RUN_MIGRATIONS=true` → `migrate.Up` at startup |
| `apps/api/migrations/0008_profile_content.sql` | `profile.content jsonb` + object check |
| `apps/api/migrations/0009_seed_site_content.sql` | generated once: profile content, social links, site settings, 2 posts, categories, tags |
| `apps/api/internal/profile/{profile,repository,repository_admin,admin,handler_admin}.go` | read/write `content` |
| `apps/api/Dockerfile`, `.github/workflows/api.yml` | Go 1.25; CI = test+lint only |
| `render.yaml` | Render Blueprint for the API (sets `RUN_MIGRATIONS=true`) |
| `.github/workflows/keepalive.yml` | 10-min healthz ping |
| `apps/web/lib/types.ts`, `apps/web/app/admin/profile/profile-form.tsx` | JSON editor for `content` |
| `apps/web/e2e/profile-content.spec.ts` | e2e for the editor |
| `design.md`, `.hallmark/log.json` | locked admin design system (Hallmark, Cobalt) |
| `apps/web/app/{globals.css,layout.tsx,providers.tsx}` | Cobalt tokens light/dark, Space Grotesk + Inter + JetBrains Mono, next-themes |
| `apps/web/app/admin/{nav.ts,sidebar.tsx,layout.tsx,command-palette.tsx,user-menu.tsx,page.tsx}` | grouped rail, top bar, ⌘K palette, theme switch, Desk dashboard |
| `apps/web/components/{page-header,status-chip}.tsx` | shared page header and post status chip |
| `apps/web/{wrangler.jsonc,open-next.config.ts,public/_headers,next.config.ts,package.json,.gitignore}` | OpenNext on Workers |
| `apps/web/app/page.tsx` | redirect `/` → `/admin` |
| `.github/workflows/web.yml` | PR checks + deploy to Cloudflare on `main` |
| `apps/site/**` | copy of `quocdaijr/qdjr@4f37a44`, pruned |
| `apps/site/app/plugins/api.ts` | `$api` client for `/v1/*` |
| `apps/site/app/utils/{profileContent,markdown}.ts` | pure adapters (unit-tested) |
| `apps/site/app/composables/{useBlogPosts,useProfile,useSiteSettings}.ts` | data access for pages |
| `apps/site/app/app.vue` | loads profile + settings once per request (`site-shell`) |
| `apps/site/app/pages/blog/**`, `app/components/SocialLinks.vue` | render from API shapes |
| `netlify.toml` | Netlify builds `apps/site` |
| `.claude/commands/{db-reset,migration,deploy-api}.md`, `infra/dns/README.md`, `docs/superpowers/setup/2026-10-12-deployment-checklist.md`, `CLAUDE.md`, `README.md` | docs for goose and the new platforms |

---

### Task 0: Mirror this plan into the repo

**Files:**
- Create: `docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md`

- [ ] **Step 1: Copy the plan file and branch**

```bash
cp ~/.claude/plans/short-summary-repo-for-witty-flamingo.md docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md
git checkout -b feature/dainq/goose-render-cloudflare-site-cutover
git add docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md
git commit -m "docs: plan goose migrations, Render/Cloudflare hosting and apps/site API cutover"
```

**Done when:**
- [ ] `docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md` is byte-identical to the plan-mode file (`cmp` exits 0)
- [ ] branch `feature/dainq/goose-render-cloudflare-site-cutover` exists with exactly one new commit

---

### Task 1: goose-managed migrations embedded in the API

**Files:**
- Create: `apps/api/migrations/embed.go`, `apps/api/migrations/0001_init_enums.sql` … `0006_indexes.sql` (converted from `supabase/migrations/`), `apps/api/migrations/0007_seed_roles_and_singletons.sql` (from `supabase/seed.sql`), `apps/api/migrations/migrations_test.go`, `apps/api/internal/migrate/migrate.go`, `apps/api/internal/migrate/migrate_test.go`, `apps/api/cmd/migrate/main.go`
- Modify: `apps/api/internal/config/config.go:10-19, 24-33`, `apps/api/cmd/api/main.go:16-29, 62-66`, `apps/api/.env.example:9`, `supabase/config.toml:65`, `apps/api/internal/contact/admin.go:15`, `apps/api/internal/profile/repository.go:27-29`, `.claude/commands/db-reset.md`, `.claude/commands/migration.md`
- Delete: `supabase/migrations/`, `supabase/seed.sql`, `.github/workflows/migrations.yml`

**Interfaces:**
- Produces: `migrations.FS embed.FS`; `migrate.Commands map[string]bool`; `migrate.Run(ctx context.Context, dsn, command string, args ...string) error`; `migrate.Up(ctx context.Context, dsn string) error`; `config.Config.RunMigrations bool` (env `RUN_MIGRATIONS=true`); CLI `go run ./cmd/migrate [up|up-by-one|up-to N|status|version]` reading `DATABASE_URL`; goose version table `public.goose_db_version`.

- [ ] **Step 1: Failing tests**

`apps/api/migrations/migrations_test.go`:

```go
package migrations

import (
	"math"
	"testing"

	"github.com/pressly/goose/v3"
)

// Versions must be unique and contiguous from 1: a duplicate or skipped
// number is a review mistake that goose would otherwise only surface against
// a real database.
func TestEmbeddedMigrationsAreContiguous(t *testing.T) {
	goose.SetBaseFS(FS)
	ms, err := goose.CollectMigrations(".", 0, math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 7 {
		t.Fatalf("expected at least 7 migrations, got %d", len(ms))
	}
	for i, m := range ms {
		if m.Version != int64(i+1) {
			t.Fatalf("%s has version %d, expected %d (numbering must be contiguous)", m.Source, m.Version, i+1)
		}
	}
}
```

`apps/api/internal/migrate/migrate_test.go`:

```go
package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_RefusesDestructiveCommands(t *testing.T) {
	for _, cmd := range []string{"down", "down-to", "reset", "redo", "create", "fix", ""} {
		err := Run(context.Background(), "postgres://unused", cmd)
		require.Error(t, err, cmd)
		assert.Contains(t, err.Error(), "unsupported command", cmd)
	}
}

func TestUp_IsIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx := context.Background()
	require.NoError(t, Up(ctx, dsn))
	require.NoError(t, Up(ctx, dsn)) // second run: nothing to apply, no error
	require.NoError(t, Run(ctx, dsn, "status"))
}
```

- [ ] **Step 2: Run them to see them fail**

```bash
cd apps/api && go test ./migrations/ ./internal/migrate/ 2>&1 | head -5
```
Expected: `no Go files` / `cannot find package` errors.

- [ ] **Step 3: Move the SQL files under goose**

```bash
cd apps/api
go get github.com/pressly/goose/v3@v3.29.0
mkdir -p migrations
for f in ../../supabase/migrations/*.sql; do
  n=$(basename "$f")
  { echo "-- apps/api/migrations/$n"; echo "-- +goose Up"; grep -v '^-- supabase/migrations/' "$f"; } > "migrations/$n"
done
{ echo "-- apps/api/migrations/0007_seed_roles_and_singletons.sql"; echo "-- +goose Up"; grep -v '^-- supabase/seed.sql' ../../supabase/seed.sql; } > migrations/0007_seed_roles_and_singletons.sql
ls migrations; head -3 migrations/0001_init_enums.sql; head -4 migrations/0007_seed_roles_and_singletons.sql
cd ../..
git rm -rq supabase/migrations supabase/seed.sql .github/workflows/migrations.yml
```
Expected listing: `0001_init_enums.sql … 0006_indexes.sql 0007_seed_roles_and_singletons.sql`; each file's line 2 is `-- +goose Up`. (No existing file has `$$` bodies or functions — verified — so no `StatementBegin` wrappers are needed yet.)

`apps/api/migrations/embed.go`:

```go
// Package migrations embeds the schema migrations so the API binary (and
// cmd/migrate) carry them. goose applies them in numeric order; see
// internal/migrate.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 4: The migrate package and CLI**

`apps/api/internal/migrate/migrate.go`:

```go
// Package migrate applies the embedded SQL migrations (apps/api/migrations)
// with goose. cmd/api calls Up at startup when RUN_MIGRATIONS=true; cmd/migrate
// exposes a few commands for local use. Forward-only: nothing here can roll a
// database back.
package migrate

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/quocdaijr/qdjr-admin/apps/api/migrations"
)

// Commands is the allowlist cmd/migrate accepts. down/reset/redo are left out
// on purpose: rollback in this repo is a new migration, never a reversal.
var Commands = map[string]bool{"up": true, "up-by-one": true, "up-to": true, "status": true, "version": true}

// Run executes one goose command against dsn using the embedded migrations.
func Run(ctx context.Context, dsn, command string, args ...string) error {
	if !Commands[command] {
		return fmt.Errorf("unsupported command %q (use up, up-by-one, up-to N, status, version)", command)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.RunContext(ctx, command, db, ".", args...)
}

// Up applies every pending migration.
func Up(ctx context.Context, dsn string) error {
	return Run(ctx, dsn, "up")
}
```

`apps/api/cmd/migrate/main.go`:

```go
// apps/api/cmd/migrate/main.go
//
// Manual migration runner:
//
//	go run ./cmd/migrate            # up
//	go run ./cmd/migrate status
//	go run ./cmd/migrate up-to 7
//
// Reads DATABASE_URL like the API. In production the API applies migrations
// itself at startup (RUN_MIGRATIONS=true), so this is for local databases and
// emergencies only.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/migrate"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"up"}
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "migrate: DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := migrate.Run(ctx, dsn, args[0], args[1:]...); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run at API startup behind `RUN_MIGRATIONS`**

FIND `apps/api/internal/config/config.go:10-19`:

```go
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
```
REPLACE with:
```go
type Config struct {
	Port                   string
	Env                    string
	DatabaseURL            string
	SupabaseURL            string
	SupabaseJWTSecret      string
	SupabaseJWKSURL        string
	SupabaseServiceRoleKey string
	CORSOrigins            []string
	// RunMigrations applies embedded goose migrations before serving
	// (RUN_MIGRATIONS=true). On in production, off locally.
	RunMigrations bool
}
```

FIND `config.go:32-33`:
```go
		CORSOrigins:            parseCSV(get("CORS_ORIGINS")),
	}
```
REPLACE with:
```go
		CORSOrigins:            parseCSV(get("CORS_ORIGINS")),
		RunMigrations:          get("RUN_MIGRATIONS") == "true",
	}
```

FIND `apps/api/cmd/api/main.go:62-66`:

```go
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()
```
REPLACE with:
```go
	if cfg.RunMigrations {
		if err := migrate.Up(ctx, cfg.DatabaseURL); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		log.Info("migrations applied")
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()
```

FIND `main.go:23-24`:
```go
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/media"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
```
REPLACE with:
```go
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/media"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/migrate"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
```

FIND `apps/api/.env.example:9` `DATABASE_URL=postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable` → REPLACE with:
```
DATABASE_URL=postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable
# Apply embedded goose migrations before serving. true on Render; locally use `go run ./cmd/migrate`.
RUN_MIGRATIONS=false
```

Comment fixes: `apps/api/internal/contact/admin.go:15` replace `supabase/migrations/0001_init_enums.sql` with `apps/api/migrations/0001_init_enums.sql`; `apps/api/internal/profile/repository.go:27-29` replace `supabase/seed.sql` with `apps/api/migrations/0007_seed_roles_and_singletons.sql`.

- [ ] **Step 6: Supabase CLI keeps only the local Postgres**

FIND `supabase/config.toml:65` `sql_paths = ["./seed.sql"]` → REPLACE `sql_paths = []`.

Replace `.claude/commands/db-reset.md`:

````markdown
---
description: Reset the local Supabase database and reapply all goose migrations
---

```bash
supabase db reset && cd apps/api && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up
```

Drops the local DB to an empty schema, then applies every file in `apps/api/migrations/` in order (goose; version table `public.goose_db_version`). Safe any time during development; never run against production.
````

Replace `.claude/commands/migration.md`:

````markdown
---
description: Create a new goose migration file
argument-hint: <name>
---

Create the next sequential file in `apps/api/migrations/` (forward-only, no Down section):

```bash
cd apps/api/migrations && next=$(printf '%04d' $(( 10#$(ls | grep -o '^[0-9]\{4\}' | sort | tail -1) + 1 ))) && printf -- '-- apps/api/migrations/%s_%s.sql\n-- +goose Up\n' "$next" "$1" > "${next}_$1.sql" && echo "${next}_$1.sql"
```

Wrap any statement that contains `;` inside strings or dollar-quotes in `-- +goose StatementBegin` / `-- +goose StatementEnd`. Apply locally with `/db-reset` (or `go run ./cmd/migrate up`); production applies it on the next API deploy (`RUN_MIGRATIONS=true`). `go test ./migrations/` fails on gaps or duplicate numbers.
````

- [ ] **Step 7: Verify against a fresh local database**

```bash
supabase start >/dev/null; supabase db reset 2>&1 | tail -2
cd apps/api && go build ./... && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up 2>&1 | tail -3
env $(grep -v '^#' .env | xargs) go run ./cmd/migrate status 2>&1 | tail -3
env $(grep -v '^#' .env | xargs) go run ./cmd/migrate down; echo "exit=$?"
psql 'postgres://postgres:postgres@127.0.0.1:54322/postgres' -At -c "select count(*) from public.roles; select max(version_id) from public.goose_db_version"
TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable' go test -race ./... 2>&1 | tail -4
cd ../..
```
Expected: `db reset` ends with `Finished supabase db reset` and no migrations applied; `migrate up` prints `OK   0001_init_enums.sql` … `goose: successfully migrated database to version: 7`; `status` shows all seven `Applied At`; `down` prints `unsupported command "down"` and `exit=1`; psql prints `3` then `7`; every package `ok` (DB-backed suites now run instead of skipping). Trap: if `db reset` still replays old migrations, the `supabase/migrations` dir was not removed.

- [ ] **Step 8: Commit**

```bash
git add -A apps/api/migrations apps/api/internal/migrate apps/api/cmd/migrate apps/api/internal/config apps/api/cmd/api/main.go apps/api/go.mod apps/api/go.sum apps/api/.env.example apps/api/internal/contact/admin.go apps/api/internal/profile/repository.go supabase/config.toml .claude/commands
git commit -m "feat(api): goose migrations embedded in the binary, applied at startup"
```

**Done when:**
- [ ] `supabase/migrations/`, `supabase/seed.sql`, `.github/workflows/migrations.yml` are gone
- [ ] `apps/api/migrations/0001…0007` each start with the path comment then `-- +goose Up`
- [ ] `go run ./cmd/migrate up` on a fresh `supabase db reset` reaches version 7; a second run is a no-op
- [ ] `go run ./cmd/migrate down` exits 1 with `unsupported command`
- [ ] `TestEmbeddedMigrationsAreContiguous` and `TestUp_IsIdempotent` pass; `go test -race ./...` green with `TEST_DATABASE_URL` set

---

### Task 2: `profile.content` in the API

**Files:**
- Create: `apps/api/migrations/0008_profile_content.sql`
- Modify: `apps/api/internal/profile/profile.go:17-27`
- Modify: `apps/api/internal/profile/admin.go:16-25`
- Modify: `apps/api/internal/profile/repository.go:30-63`
- Modify: `apps/api/internal/profile/repository_admin.go:26-57, 95-100`
- Modify: `apps/api/internal/profile/handler_admin.go:29-37, 90-101`
- Test: `apps/api/internal/profile/handler_admin_test.go` (stub at :60-77, new tests appended), `apps/api/internal/profile/repository_test.go:40-60` (fixture), `apps/api/internal/profile/repository_admin_test.go` (new test appended)

**Interfaces:**
- Produces: `Profile.Content json.RawMessage` (always a JSON object, `{}` when unset) on `GET /v1/profile` and `GET /v1/admin/profile`; `PATCH /v1/admin/profile` accepts `"content": {…}`; `UpdateInput.Content *json.RawMessage`; `parseContent(raw json.RawMessage) (*json.RawMessage, error)`; `normalizeContent(raw []byte) json.RawMessage`.

- [ ] **Step 1: Write the migration**

```sql
-- apps/api/migrations/0008_profile_content.sql
-- +goose Up
-- Rich bilingual site copy rendered by apps/site on / and /about:
-- {"vi": ProfileContent, "en": ProfileContent}. The shape is owned by
-- apps/site/app/data/profile.ts; the API stores and serves it opaquely.
alter table public.profile
    add column content jsonb not null default '{}'::jsonb,
    add constraint profile_content_is_object check (jsonb_typeof(content) = 'object');
```

- [ ] **Step 2: Write the failing handler tests**

Append to `apps/api/internal/profile/handler_admin_test.go`:

```go
func TestAdminProfile_Update_Content_OK(t *testing.T) {
	env := newAdminEnv("profile:write")
	body := `{"content":{"vi":{"profile":{"name":"Đại"}},"en":{}}}`
	rr := env.doRaw(t, http.MethodPatch, "/v1/admin/profile", body)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.True(t, env.repo.updated)
	require.NotNil(t, env.repo.updateIn.Content)
	assert.JSONEq(t, `{"vi":{"profile":{"name":"Đại"}},"en":{}}`, string(*env.repo.updateIn.Content))

	var resp struct {
		Data Profile `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.JSONEq(t, `{"vi":{"profile":{"name":"Đại"}},"en":{}}`, string(resp.Data.Content))
}

func TestAdminProfile_Update_Content_RejectsNonObject(t *testing.T) {
	for _, raw := range []string{`{"content":[1,2]}`, `{"content":"x"}`, `{"content":null}`, `{"content":7}`} {
		env := newAdminEnv("profile:write")
		rr := env.doRaw(t, http.MethodPatch, "/v1/admin/profile", raw)
		assert.Equal(t, http.StatusBadRequest, rr.Code, raw)
		assert.False(t, env.repo.updated, raw)
	}
}

func TestParseContent(t *testing.T) {
	got, err := parseContent(nil)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = parseContent(json.RawMessage(`{"a":1}`))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.JSONEq(t, `{"a":1}`, string(*got))

	_, err = parseContent(json.RawMessage(`null`))
	assert.Error(t, err)
}
```

Update the stub so the first test can observe the write. FIND in `handler_admin_test.go:70-77`:

```go
	if in.SocialLinks != nil {
		m := *in.SocialLinks
		cp := make(map[string]string, len(m))
		for k, v := range m {
			cp[k] = v
		}
		s.current.SocialLinks = cp
	}
	return s.current, nil
```

REPLACE with:

```go
	if in.SocialLinks != nil {
		m := *in.SocialLinks
		cp := make(map[string]string, len(m))
		for k, v := range m {
			cp[k] = v
		}
		s.current.SocialLinks = cp
	}
	if in.Content != nil {
		s.current.Content = append(json.RawMessage(nil), (*in.Content)...)
	}
	return s.current, nil
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd apps/api && go test ./internal/profile/ -run 'TestAdminProfile_Update_Content|TestParseContent' 2>&1 | head -20
```
Expected: compile error `in.Content undefined` / `undefined: parseContent`.

- [ ] **Step 4: Add the field and helper (`profile.go`)**

FIND `apps/api/internal/profile/profile.go:7-27`:

```go
import (
	"context"
	"time"
)

// Profile is the public response shape for the singleton profile row.
//
// All optional text columns are returned as JSON null when absent. AvatarURL
// is the resolved public URL (storage prefix + media.storage_path) when an
// avatar is linked, otherwise nil.
type Profile struct {
	ID          int16             `json:"id"`
	FullName    *string           `json:"full_name"`
	Bio         *string           `json:"bio"`
	AvatarURL   *string           `json:"avatar_url"`
	Tagline     *string           `json:"tagline"`
	SocialLinks map[string]string `json:"social_links"`
	Location    *string           `json:"location"`
	Email       *string           `json:"email"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
```

REPLACE with:

```go
import (
	"context"
	"encoding/json"
	"time"
)

// Profile is the public response shape for the singleton profile row.
//
// All optional text columns are returned as JSON null when absent. AvatarURL
// is the resolved public URL (storage prefix + media.storage_path) when an
// avatar is linked, otherwise nil. Content is the bilingual site copy blob
// ({"vi": …, "en": …}) owned by apps/site; always a JSON object, {} when unset.
type Profile struct {
	ID          int16             `json:"id"`
	FullName    *string           `json:"full_name"`
	Bio         *string           `json:"bio"`
	AvatarURL   *string           `json:"avatar_url"`
	Tagline     *string           `json:"tagline"`
	SocialLinks map[string]string `json:"social_links"`
	Location    *string           `json:"location"`
	Email       *string           `json:"email"`
	Content     json.RawMessage   `json:"content"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// normalizeContent turns a scanned jsonb value into a JSON object, so the
// API never serves "null" for content.
func normalizeContent(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}
```

- [ ] **Step 5: Extend `UpdateInput` (`admin.go`)**

FIND `apps/api/internal/profile/admin.go:16-25`:

```go
type UpdateInput struct {
	FullName    *string
	Bio         *string
	AvatarID    **uuid.UUID
	Tagline     *string
	SocialLinks *map[string]string
	Location    *string
	Email       *string
	UpdatedBy   uuid.UUID
}
```

REPLACE with:

```go
type UpdateInput struct {
	FullName    *string
	Bio         *string
	AvatarID    **uuid.UUID
	Tagline     *string
	SocialLinks *map[string]string
	Location    *string
	Email       *string
	Content     *json.RawMessage // nil = unchanged; otherwise a JSON object
	UpdatedBy   uuid.UUID
}
```

and FIND `admin.go:3-7`:

```go
import (
	"context"

	"github.com/google/uuid"
)
```

REPLACE with:

```go
import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)
```

- [ ] **Step 6: Read `content` in both repositories**

FIND `apps/api/internal/profile/repository.go:31-46`:

```go
	const q = `
        SELECT p.id, p.full_name, p.bio, m.storage_path, p.tagline,
               p.social_links, p.location, p.email, p.updated_at
        FROM public.profile p
        LEFT JOIN public.media m ON m.id = p.avatar_id
        WHERE p.id = 1`

	var (
		out         Profile
		avatarPath  *string
		socialBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.ID, &out.FullName, &out.Bio, &avatarPath, &out.Tagline,
		&socialBytes, &out.Location, &out.Email, &out.UpdatedAt,
	)
```

REPLACE with:

```go
	const q = `
        SELECT p.id, p.full_name, p.bio, m.storage_path, p.tagline,
               p.social_links, p.location, p.email, p.content, p.updated_at
        FROM public.profile p
        LEFT JOIN public.media m ON m.id = p.avatar_id
        WHERE p.id = 1`

	var (
		out          Profile
		avatarPath   *string
		socialBytes  []byte
		contentBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.ID, &out.FullName, &out.Bio, &avatarPath, &out.Tagline,
		&socialBytes, &out.Location, &out.Email, &contentBytes, &out.UpdatedAt,
	)
```

FIND `repository.go:56-62`:

```go
	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Profile{}, fmt.Errorf("profile decode social_links: %w", err)
		}
	}
	return out, nil
```

REPLACE with:

```go
	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Profile{}, fmt.Errorf("profile decode social_links: %w", err)
		}
	}
	out.Content = normalizeContent(contentBytes)
	return out, nil
```

FIND `apps/api/internal/profile/repository_admin.go:27-57`:

```go
	const q = `
        SELECT p.id, p.full_name, p.bio, m.storage_path, p.tagline,
               p.social_links, p.location, p.email, p.updated_at
        FROM public.profile p
        LEFT JOIN public.media m ON m.id = p.avatar_id
        WHERE p.id = 1`

	var (
		out         Profile
		avatarPath  *string
		socialBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.ID, &out.FullName, &out.Bio, &avatarPath, &out.Tagline,
		&socialBytes, &out.Location, &out.Email, &out.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("admin profile get: %w", err)
	}
	if avatarPath != nil {
		url := r.avatarURL(*avatarPath)
		out.AvatarURL = &url
	}
	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Profile{}, fmt.Errorf("admin profile decode social_links: %w", err)
		}
	}
	return out, nil
}
```

REPLACE with:

```go
	const q = `
        SELECT p.id, p.full_name, p.bio, m.storage_path, p.tagline,
               p.social_links, p.location, p.email, p.content, p.updated_at
        FROM public.profile p
        LEFT JOIN public.media m ON m.id = p.avatar_id
        WHERE p.id = 1`

	var (
		out          Profile
		avatarPath   *string
		socialBytes  []byte
		contentBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.ID, &out.FullName, &out.Bio, &avatarPath, &out.Tagline,
		&socialBytes, &out.Location, &out.Email, &contentBytes, &out.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("admin profile get: %w", err)
	}
	if avatarPath != nil {
		url := r.avatarURL(*avatarPath)
		out.AvatarURL = &url
	}
	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Profile{}, fmt.Errorf("admin profile decode social_links: %w", err)
		}
	}
	out.Content = normalizeContent(contentBytes)
	return out, nil
}
```

- [ ] **Step 7: Write `content` in `AdminRepository.Update`**

FIND `repository_admin.go:95-101`:

```go
	if in.Location != nil {
		add("location", "", *in.Location)
	}
	if in.Email != nil {
		add("email", "", *in.Email)
	}
	add("updated_by", "", in.UpdatedBy)
```

REPLACE with:

```go
	if in.Location != nil {
		add("location", "", *in.Location)
	}
	if in.Email != nil {
		add("email", "", *in.Email)
	}
	if in.Content != nil {
		add("content", "jsonb", string(*in.Content))
	}
	add("updated_by", "", in.UpdatedBy)
```

- [ ] **Step 8: Accept and validate `content` in the admin handler**

FIND `handler_admin.go:29-37`:

```go
type adminUpdatePayload struct {
	FullName    *string            `json:"full_name,omitempty"`
	Bio         *string            `json:"bio,omitempty"`
	AvatarID    json.RawMessage    `json:"avatar_id,omitempty"`
	Tagline     *string            `json:"tagline,omitempty"`
	SocialLinks *map[string]string `json:"social_links,omitempty"`
	Location    *string            `json:"location,omitempty"`
	Email       *string            `json:"email,omitempty"`
}
```

REPLACE with:

```go
type adminUpdatePayload struct {
	FullName    *string            `json:"full_name,omitempty"`
	Bio         *string            `json:"bio,omitempty"`
	AvatarID    json.RawMessage    `json:"avatar_id,omitempty"`
	Tagline     *string            `json:"tagline,omitempty"`
	SocialLinks *map[string]string `json:"social_links,omitempty"`
	Location    *string            `json:"location,omitempty"`
	Email       *string            `json:"email,omitempty"`
	Content     json.RawMessage    `json:"content,omitempty"`
}

// parseContent accepts an absent key (nil, nil) or a JSON object. null,
// arrays and scalars are rejected so the DB check constraint is never the
// first thing to complain.
func parseContent(raw json.RawMessage) (*json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, errors.New("content must be a JSON object")
	}
	out := append(json.RawMessage(nil), raw...)
	return &out, nil
}
```

FIND `handler_admin.go:85-101`:

```go
		avatarValue, avatarSet, err := parseAvatarID(p.AvatarID)
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid avatar_id: "+err.Error())
			return
		}
		in := UpdateInput{
			FullName:    p.FullName,
			Bio:         p.Bio,
			Tagline:     p.Tagline,
			SocialLinks: p.SocialLinks,
			Location:    p.Location,
			Email:       p.Email,
			UpdatedBy:   uid,
		}
		if avatarSet {
			in.AvatarID = &avatarValue
		}
```

REPLACE with:

```go
		avatarValue, avatarSet, err := parseAvatarID(p.AvatarID)
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid avatar_id: "+err.Error())
			return
		}
		content, err := parseContent(p.Content)
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		in := UpdateInput{
			FullName:    p.FullName,
			Bio:         p.Bio,
			Tagline:     p.Tagline,
			SocialLinks: p.SocialLinks,
			Location:    p.Location,
			Email:       p.Email,
			Content:     content,
			UpdatedBy:   uid,
		}
		if avatarSet {
			in.AvatarID = &avatarValue
		}
```

and FIND `handler_admin.go:3-5`:

```go
import (
	"encoding/json"
	"net/http"
```

REPLACE with:

```go
import (
	"encoding/json"
	"errors"
	"net/http"
```

- [ ] **Step 9: Run the handler tests**

```bash
cd apps/api && go test ./internal/profile/ 2>&1 | tail -5
```
Expected: `ok  	github.com/quocdaijr/qdjr-admin/apps/api/internal/profile`.

- [ ] **Step 10: Add the DB-backed test and extend the fixture**

FIND `repository_test.go:40-60`:

```go
	// Snapshot current row so we can restore it after the test.
	var (
		fullName, bio, tagline, location, email *string
		avatarID                                *uuid.UUID
		socialLinks                             []byte
	)
	err = pool.QueryRow(ctx,
		`select full_name, bio, avatar_id, tagline, social_links, location, email
		   from public.profile where id = 1`,
	).Scan(&fullName, &bio, &avatarID, &tagline, &socialLinks, &location, &email)
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg,
			`update public.profile set full_name=$1, bio=$2, avatar_id=$3,
			   tagline=$4, social_links=$5::jsonb, location=$6, email=$7
			 where id = 1`,
			fullName, bio, avatarID, tagline, socialLinks, location, email,
		)
	})
```

REPLACE with:

```go
	// Snapshot current row so we can restore it after the test.
	var (
		fullName, bio, tagline, location, email *string
		avatarID                                *uuid.UUID
		socialLinks, content                    []byte
	)
	err = pool.QueryRow(ctx,
		`select full_name, bio, avatar_id, tagline, social_links, location, email, content
		   from public.profile where id = 1`,
	).Scan(&fullName, &bio, &avatarID, &tagline, &socialLinks, &location, &email, &content)
	require.NoError(t, err)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg,
			`update public.profile set full_name=$1, bio=$2, avatar_id=$3,
			   tagline=$4, social_links=$5::jsonb, location=$6, email=$7, content=$8::jsonb
			 where id = 1`,
			fullName, bio, avatarID, tagline, socialLinks, location, email, content,
		)
	})
```

Append to `repository_admin_test.go` (add `"encoding/json"` to its imports):

```go
func TestAdminRepository_Update_Content_RoundTrips(t *testing.T) {
	f := newFixture(t)
	repo := NewAdminRepository(f.pool, "")

	content := json.RawMessage(`{"vi":{"quote":{"text":"x","author":"y"}},"en":{}}`)
	got, err := repo.Update(f.ctx, UpdateInput{Content: &content, UpdatedBy: f.authorID})
	require.NoError(t, err)
	assert.JSONEq(t, string(content), string(got.Content))

	pub, err := NewRepository(f.pool, "").Get(f.ctx)
	require.NoError(t, err)
	assert.JSONEq(t, string(content), string(pub.Content))
}
```

- [ ] **Step 11: Apply the migration locally and run the DB tests**

```bash
cd apps/api && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up 2>&1 | tail -2
TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable' go test ./internal/profile/ ./migrations/ -run 'Repository|Contiguous' -v 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | tail -15
cd ../..
```
Expected: `OK   0008_profile_content.sql`, `… migrated database to version: 8`; every `--- PASS`, including `TestAdminRepository_Update_Content_RoundTrips`. Trap: `column p.content does not exist` means `migrate up` was skipped.

- [ ] **Step 12: Full API test + commit**

```bash
cd apps/api && go test -race ./... 2>&1 | tail -3 && go vet ./... && cd ../..
git add apps/api/migrations/0008_profile_content.sql apps/api/internal/profile
git commit -m "feat(api/profile): bilingual site content blob on the profile row"
```

**Done when:**
- [ ] `GET /v1/profile` and `GET /v1/admin/profile` include `"content":{}` on a fresh DB
- [ ] `PATCH /v1/admin/profile` with `content` as an array, string, number or null → 400 `INVALID`, repository not called
- [ ] `TestAdminRepository_Update_Content_RoundTrips` passes against local Postgres

---

### Task 3: Copy `qdjr` into `apps/site`

**Files:**
- Create: `apps/site/**` (from `quocdaijr/qdjr` `origin/main` @ `4f37a44`)
- Modify: `apps/site/package.json`, `apps/site/.env.example`, `apps/site/nuxt.config.ts:31,60,72,80,149`

**Interfaces:**
- Produces: `apps/site` builds and its unit tests pass unchanged; no husky/pm2/docker leftovers; `npm` stays the package manager there (Netlify picks `package-lock.json`).

- [ ] **Step 1: Export a clean tree from the merged `main` of the old repo**

```bash
git -C ../qdjr fetch -q origin && git -C ../qdjr log --oneline -1 origin/main
mkdir -p apps/site
git -C ../qdjr archive origin/main | tar -x -C apps/site
ls apps/site | head -30
```
Expected: first line `4f37a44 Merge pull request #28 …`; the listing shows `app content i18n nuxt.config.ts package.json public server scripts test e2e trips …` and **no** `node_modules`, `.nuxt`, `.output`, `.data`.

- [ ] **Step 2: Prune self-hosting and hook files**

```bash
cd apps/site
rm -rf docker docker-compose.yml pm2.config.js.example bin .husky commitlint.config.js .idea .claude .hallmark .superpowers
cd ../..
```

Then edit `apps/site/package.json`. FIND:

```json
    "lint:style": "stylelint \"**/*.{vue,css}\" --ignore-path .gitignore",
    "prepare": "husky",
    "lint": "npm run lint:js && npm run lint:style",
    "copy:env": "cp .env.example .env",
    "copy:pm2.config": "cp pm2.config.js.example pm2.config.js",
    "test": "vitest run",
```
REPLACE:
```json
    "lint:style": "stylelint \"**/*.{vue,css}\" --ignore-path .gitignore",
    "lint": "npm run lint:js && npm run lint:style",
    "copy:env": "cp .env.example .env",
    "test": "vitest run",
```
FIND:
```json
  "lint-staged": {
    "*.{js,ts,vue}": "eslint --fix",
    "*.{css,vue}": "stylelint --fix"
  },
```
REPLACE with nothing (delete the four lines).
FIND:
```json
    "@commitlint/cli": "^20.5.3",
    "@commitlint/config-conventional": "^20.5.3",
    "@nuxt/devtools": "^2.4.1",
```
REPLACE:
```json
    "@nuxt/devtools": "^2.4.1",
```
FIND:
```json
    "happy-dom": "^20.11.2",
    "husky": "^9.1.7",
    "lint-staged": "^16.4.0",
    "postcss": "^8.3.10",
```
REPLACE:
```json
    "happy-dom": "^20.11.2",
    "postcss": "^8.3.10",
```
Verify: `node -e "const p=require('./apps/site/package.json');console.log(!!p['lint-staged'],!!p.scripts.prepare,!!p.devDependencies.husky)"` → `false false false`.

- [ ] **Step 3: Point defaults at qdjr.dev**

```bash
sed -i '' "s#process.env.APP_URL || 'https://qdjr.me'#process.env.APP_URL || 'https://qdjr.dev'#g" apps/site/nuxt.config.ts
grep -c "https://qdjr.dev'" apps/site/nuxt.config.ts; grep -c "qdjr.me" apps/site/nuxt.config.ts
```
Expected: `5` then `0` (lines 31, 60, 72, 80, 149).

Replace `apps/site/.env.example` entirely:

```bash
APP_URL=https://qdjr.dev

# qdjr-admin API (apps/api). Local: http://localhost:8080
API_URL=https://api.qdjr.dev

# Google Analytics measurement ID (optional; falls back to the built-in default)
# GOOGLE_ANALYTICS_ID=G-XXXXXXXXXX
```

- [ ] **Step 4: Install, test, build**

```bash
cd apps/site && npm install 2>&1 | tail -2 && npm test 2>&1 | tail -4 && npm run build 2>&1 | tail -3; cd ../..
```
Expected: `Test Files  N passed`, build ends with `✔ You can preview this build using node .output/server/index.mjs`. Trap: `npm install` regenerates `package-lock.json` after the devDependency removals; that's intended, commit it.

- [ ] **Step 5: Commit**

```bash
git add apps/site
git commit -m "feat(site): import qdjr (Nuxt 4) as apps/site"
```

**Done when:**
- [ ] `apps/site` builds (`npm run build`) and `npm test` is green, unchanged from the old repo
- [ ] no `husky`, `lint-staged`, `commitlint`, `pm2`, `docker` leftovers in `apps/site`
- [ ] `grep -c qdjr.me apps/site/nuxt.config.ts` → `0`

---

### Task 4: Seed the site's content into Postgres (migration 0009)

**Files:**
- Create (temporary, untracked): `apps/site/scripts/export-seed.ts`
- Create (generated, committed): `apps/api/migrations/0009_seed_site_content.sql`

**Interfaces:**
- Consumes: `apps/site/app/data/profile.ts` (`PROFILE_CONTENT`), `apps/site/content/blog/*.md`, the 7 hrefs in `apps/site/app/components/SocialLinks.vue:18-61`.
- Produces: rows: `profile.content = {"vi":…, "en":…}`, `profile.social_links` and `site_settings.social_links` = `{mail, telegram, signal, discord, github, facebook, linkedin}`, categories `meta`, `tools`, tags `nuxt markdown workflow vietnamese claude-code ai`, posts `hello-world`, `claude-code-tro-ly-lap-trinh` (published, markdown body in `content`, `excerpt` = front-matter description).

- [ ] **Step 1: Write the generator**

`apps/site/scripts/export-seed.ts`:

```ts
// One-off: turn apps/site's bundled content into apps/api/migrations/0009_seed_site_content.sql.
// Run from apps/site:  npx -y tsx scripts/export-seed.ts
import {readFileSync, readdirSync, writeFileSync} from 'node:fs'
import {basename, join} from 'node:path'
import {parse as parseYaml} from 'yaml'
import {PROFILE_CONTENT} from '../app/data/profile'

const SOCIAL_LINKS: Record<string, string> = {
  mail: 'mailto:quocdaijr@gmail.com',
  telegram: 'https://t.me/quocdaijr',
  signal: 'https://signal.me/#eu/U1eIvUn29-2TB8fu6lTq-nxX0qOa3LNUCoyywDx_WVvEdYF7C5NK15eKfieS3TDe',
  discord: 'https://discord.com/users/quocdaijr',
  github: 'https://github.com/quocdaijr',
  facebook: 'https://facebook.com/quocdaijr',
  linkedin: 'https://www.linkedin.com/in/quocdaijr'
}

const lit = (s: string) => `'${s.replace(/'/g, "''")}'`
const body = (s: string) => {
  if (s.includes('$md$')) throw new Error('post body contains the $md$ quote tag')
  return `$md$${s}$md$`
}
const titleCase = (slug: string) => slug.split('-').map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(' ')
// goose splits statements on line-ending semicolons; anything multi-line or
// carrying free text is fenced so a ";" inside a string can't cut it in half.
const fenced = (statement: string) => `-- +goose StatementBegin\n${statement}\n-- +goose StatementEnd`

interface SeedPost {slug: string; title: string; description: string; publishedAt: string; category?: string; tags: string[]; body: string; draft: boolean}

const posts: SeedPost[] = readdirSync('content/blog')
  .filter((f) => f.endsWith('.md'))
  .sort()
  .map((file) => {
    const raw = readFileSync(join('content/blog', file), 'utf8')
    const m = raw.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/)
    if (!m) throw new Error(`no front matter: ${file}`)
    const fm = parseYaml(m[1]) as Record<string, unknown>
    return {
      slug: basename(file, '.md'),
      title: String(fm.title),
      description: String(fm.description ?? ''),
      publishedAt: new Date(String(fm.publishedAt)).toISOString(),
      category: fm.category ? String(fm.category) : undefined,
      tags: Array.isArray(fm.tags) ? fm.tags.map(String) : [],
      body: m[2].trim(),
      draft: Boolean(fm.draft)
    }
  })
  .filter((p) => !p.draft)

const categories = [...new Set(posts.flatMap((p) => (p.category ? [p.category] : [])))]
const tags = [...new Set(posts.flatMap((p) => p.tags))]

const out: string[] = [
  '-- apps/api/migrations/0009_seed_site_content.sql',
  '-- +goose Up',
  '-- Generated once by apps/site/scripts/export-seed.ts from the content that used to ship',
  '-- inside the Nuxt site (content/blog/*.md, app/data/profile.*.ts, SocialLinks.vue).',
  '-- Idempotent: inserts are "on conflict do nothing"; singleton updates only fill empty rows.',
  '',
  fenced(`update public.profile set
    full_name = ${lit(PROFILE_CONTENT.vi.profile.name)},
    tagline = ${lit(PROFILE_CONTENT.en.profile.role)},
    bio = ${lit(PROFILE_CONTENT.en.profile.summary)},
    email = 'quocdaijr@gmail.com',
    social_links = ${lit(JSON.stringify(SOCIAL_LINKS))}::jsonb,
    content = ${lit(JSON.stringify(PROFILE_CONTENT))}::jsonb
where id = 1 and content = '{}'::jsonb;`),
  '',
  fenced(`update public.site_settings set
    site_title = 'QDJr Blog',
    site_description = 'This is a small blog of Nguyen Quoc Dai',
    contact_email = 'quocdaijr@gmail.com',
    social_links = ${lit(JSON.stringify(SOCIAL_LINKS))}::jsonb
where id = 1 and social_links = '{}'::jsonb;`),
  ''
]

for (const c of categories) out.push(`insert into public.categories (slug, name) values (${lit(c)}, ${lit(titleCase(c))}) on conflict (slug) do nothing;`)
for (const t of tags) out.push(`insert into public.tags (slug, name) values (${lit(t)}, ${lit(titleCase(t))}) on conflict (slug) do nothing;`)
out.push('')

for (const p of posts) {
  out.push(fenced(`insert into public.posts (slug, title, excerpt, content, status, published_at)
values (${lit(p.slug)}, ${lit(p.title)}, ${lit(p.description)}, ${body(p.body)}, 'published', ${lit(p.publishedAt)}::timestamptz)
on conflict (slug) do nothing;`))
  if (p.category) {
    out.push(fenced(`insert into public.post_categories (post_id, category_id)
select p.id, c.id from public.posts p, public.categories c where p.slug = ${lit(p.slug)} and c.slug = ${lit(p.category)}
on conflict do nothing;`))
  }
  for (const t of p.tags) {
    out.push(fenced(`insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = ${lit(p.slug)} and t.slug = ${lit(t)}
on conflict do nothing;`))
  }
  out.push('')
}

writeFileSync('../api/migrations/0009_seed_site_content.sql', out.join('\n'))
console.log(`wrote ${posts.length} posts, ${categories.length} categories, ${tags.length} tags`)
```

- [ ] **Step 2: Generate and inspect**

```bash
cd apps/site && npx -y tsx scripts/export-seed.ts && cd ../..
head -2 apps/api/migrations/0009_seed_site_content.sql
grep -c "insert into public.posts" apps/api/migrations/0009_seed_site_content.sql
grep -n "^insert into public.\(categories\|tags\)" apps/api/migrations/0009_seed_site_content.sql
```
Expected: `wrote 2 posts, 2 categories, 6 tags`; line 2 is `-- +goose Up`; count `2`; category rows for `tools` and `meta`, tag rows for `claude-code ai workflow vietnamese nuxt markdown`.

- [ ] **Step 3: Apply, re-apply, verify idempotence**

```bash
cd apps/api
go test ./migrations/ 2>&1 | tail -1
env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up 2>&1 | tail -2
psql 'postgres://postgres:postgres@127.0.0.1:54322/postgres' -At -c "select (select count(*) from posts) || ' posts, ' || (select count(*) from post_tags) || ' post_tags, ' || (select jsonb_typeof(content) from profile) || ' content, ' || (select count(*) from jsonb_object_keys((select social_links from site_settings))) || ' social'"
env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up 2>&1 | tail -1
cd ../..
```
Expected: migrations test `ok`; `OK   0009_seed_site_content.sql` → version 9; `2 posts, 8 post_tags, object content, 7 social`; second `up` prints `goose: no migrations to run. current version: 9`. Trap: if `psql` is missing, use `docker exec -i supabase_db_qdjr-admin psql -U postgres` instead.

- [ ] **Step 4: Remove the generator and commit**

```bash
rm apps/site/scripts/export-seed.ts
git add apps/api/migrations/0009_seed_site_content.sql
git commit -m "feat(db): seed posts, taxonomy, profile content and social links from the site"
```

**Done when:**
- [ ] `0009_seed_site_content.sql` is committed and the generator script is not
- [ ] after `migrate up`: 2 published posts, 2 categories, 6 tags, 8 post_tags, profile.content is an object with `vi` and `en`, 7 social links
- [ ] re-running `migrate up` prints `no migrations to run`

---

### Task 5: Admin edits `content` as JSON

**Files:**
- Modify: `apps/web/lib/types.ts:66-77`
- Modify: `apps/web/app/admin/profile/profile-form.tsx:24-50, 156-164, 175-191, 193-205`
- Create: `apps/web/e2e/profile-content.spec.ts`

**Interfaces:**
- Consumes: Task 2 (`content` on GET/PATCH `/v1/admin/profile`, `GET /v1/profile`).
- Produces: a "Site content (JSON)" textarea on `/admin/profile` that round-trips `content`.

- [ ] **Step 1: Write the failing e2e**

`apps/web/e2e/profile-content.spec.ts`:

```ts
import { test, expect, type Page } from '@playwright/test';

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';
const API_BASE = process.env.E2E_API_BASE ?? 'http://localhost:8080';

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
}

test('profile content JSON round-trips to the public API', async ({ page, request }) => {
  await login(page);
  await page.goto('/admin/profile');

  const editor = page.getByLabel('Site content (JSON)');
  await expect(editor).toBeVisible();
  const original = await editor.inputValue();
  const marker = `e2e-${Math.random().toString(36).slice(2, 8)}`;
  const next = JSON.parse(original || '{}');
  next.e2e = marker;

  await editor.fill(JSON.stringify(next, null, 2));
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Profile saved')).toBeVisible();

  const res = await request.get(`${API_BASE}/v1/profile`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.data.content.e2e).toBe(marker);

  // restore
  await editor.fill(original || '{}');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Profile saved')).toBeVisible();
});

test('invalid JSON is rejected before saving', async ({ page }) => {
  await login(page);
  await page.goto('/admin/profile');
  const editor = page.getByLabel('Site content (JSON)');
  await editor.fill('[1, 2]');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Must be a JSON object')).toBeVisible();
});
```

- [ ] **Step 2: Run it to see it fail**

(API on :8080, web on :3000 — see Task 12 step 1 for starting them.)
```bash
cd apps/web && pnpm exec playwright test e2e/profile-content.spec.ts 2>&1 | tail -6
```
Expected: FAIL `getByLabel('Site content (JSON)') … not found`.

- [ ] **Step 3: Type + form**

FIND `apps/web/lib/types.ts:73-77`:

```ts
  social_links: Record<string, string>;
  location: string | null;
  email: string | null;
  updated_at: string;
}
```

REPLACE with:

```ts
  social_links: Record<string, string>;
  location: string | null;
  email: string | null;
  // Bilingual site copy rendered by apps/site ({"vi": …, "en": …}); opaque here.
  content: Record<string, unknown>;
  updated_at: string;
}
```

FIND `profile-form.tsx:35-36`:

```ts
  social_links: z.array(z.object({ key: z.string(), url: z.string() })),
});
```

REPLACE with:

```ts
  social_links: z.array(z.object({ key: z.string(), url: z.string() })),
  content: z.string().refine(isJsonObject, 'Must be a JSON object'),
});

function isJsonObject(raw: string): boolean {
  try {
    const value: unknown = JSON.parse(raw);
    return typeof value === 'object' && value !== null && !Array.isArray(value);
  } catch {
    return false;
  }
}
```

FIND `profile-form.tsx:48-49`:

```ts
    social_links: socialLinksToRows(p?.social_links),
  };
```

REPLACE with:

```ts
    social_links: socialLinksToRows(p?.social_links),
    content: JSON.stringify(p?.content ?? {}, null, 2),
  };
```

FIND `profile-form.tsx:156-164`:

```tsx
      <Field label="Social links">
        <Controller
          control={control}
          name="social_links"
          render={({ field }) => (
            <SocialLinksEditor value={field.value} onChange={field.onChange} />
          )}
        />
      </Field>
```

REPLACE with:

```tsx
      <Field label="Social links">
        <Controller
          control={control}
          name="social_links"
          render={({ field }) => (
            <SocialLinksEditor value={field.value} onChange={field.onChange} />
          )}
        />
      </Field>

      <Field label="Site content (JSON)" error={errors.content?.message} htmlFor="profile-content">
        <Textarea
          id="profile-content"
          {...register('content')}
          rows={24}
          spellCheck={false}
          className="font-mono text-xs"
        />
        <p className="text-xs text-muted-foreground">
          Copy for qdjr.dev / and /about, one object per language: {'{ "vi": {…}, "en": {…} }'}.
          Shape lives in apps/site/app/data/profile.ts.
        </p>
      </Field>
```

FIND `profile-form.tsx:175-191` (the `Field` helper):

```tsx
function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label>{label}</Label>
      {children}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}
```

REPLACE with:

```tsx
function Field({
  label,
  error,
  htmlFor,
  children,
}: {
  label: string;
  error?: string;
  htmlFor?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}
```

FIND `profile-form.tsx:203-205`:

```ts
    social_links: rowsToSocialLinks(values.social_links),
  };
}
```

REPLACE with:

```ts
    social_links: rowsToSocialLinks(values.social_links),
    content: JSON.parse(values.content) as Record<string, unknown>,
  };
}
```

- [ ] **Step 4: Typecheck, lint, e2e**

```bash
cd apps/web && pnpm exec tsc --noEmit && pnpm lint && pnpm exec playwright test e2e/profile-content.spec.ts 2>&1 | tail -4
```
Expected: `2 passed`.

- [ ] **Step 5: Commit**

```bash
git add apps/web/lib/types.ts apps/web/app/admin/profile/profile-form.tsx apps/web/e2e/profile-content.spec.ts
git commit -m "feat(web/profile): JSON editor for the site content blob"
```

**Done when:**
- [ ] `/admin/profile` shows a "Site content (JSON)" textarea filled with the seeded blob
- [ ] saving a changed blob is visible in `GET /v1/profile`; `[1,2]` is rejected client-side
- [ ] `e2e/profile-content.spec.ts` 2 passed; `tsc` and `lint` clean

---

### Task 5A: Admin redesign (Hallmark · modern-minimal · Cobalt)

Decided with the user on 2026-10-12: **audience** = one owner who writes and publishes, so the first screen is a writing desk; **tone** = technical instrument panel (Cobalt); **dark mode** = light + dark, follow system. Hallmark multi-page flow: one locked system in `design.md`, every admin route reads it, consistency over variety.

**Hallmark preview**
- **Genre** · modern-minimal (app family)
- **Macrostructure** · Workbench for every app page: grouped left rail, top bar with an inline ⌘K pill, content column max 1200px
- **Theme** · Cobalt: cool near-white paper `oklch(98.5% 0.004 250)` · cool charcoal ink · one electric cobalt signal `oklch(54% 0.20 258)` · Space Grotesk display + Inter body + JetBrains Mono labels · 6px radii · hairlines, no shadows · graphite surface for code (the JSON editor)
- **Nav** · grouped side rail (app adaptation of N3: mono group labels, 2px cobalt tick on the active item) plus N13 inline ⌘K pill in the top bar. **Footer** · none, it is an app shell.
- **Enrichment** · none, typography only
- **Sections** · Shell (rail · top bar · palette) · Desk dashboard (drafts in progress · inbox · recently published) · list pages (PageHeader + table) · post editor (writing column + sticky meta rail)
- **Motion** · palette open fade (≤160ms) · focus ring instant · nothing else. Reduced motion: no transforms.
- **Slop test** · run in Step 9 below
- **Diversification** · inverted (`designed-as-app`): all pages share theme, type and accent

Pre-flight bug this task also fixes: `apps/web/app/globals.css:10` maps `--font-sans` to itself, so Geist never applied and every screen used the browser default sans.

**Files:**
- Create: `design.md` (repo root), `.hallmark/log.json`, `apps/web/app/admin/nav.ts`, `apps/web/app/admin/command-palette.tsx`, `apps/web/components/page-header.tsx`, `apps/web/components/status-chip.tsx`
- Modify: `apps/web/app/globals.css:7-12, 51-118`, `apps/web/app/layout.tsx`, `apps/web/app/providers.tsx`, `apps/web/app/admin/layout.tsx`, `apps/web/app/admin/sidebar.tsx`, `apps/web/app/admin/user-menu.tsx`, `apps/web/app/admin/page.tsx`, `apps/web/app/admin/posts/page.tsx`, `apps/web/app/admin/posts/posts-table.tsx`, `apps/web/app/admin/posts/post-form.tsx:167-185, 186-187, 236`, `apps/web/app/admin/profile/profile-form.tsx` (the JSON textarea from Task 5), the `<h1>` of `app/admin/{categories,tags,media,profile,settings,inbox,users}/page.tsx`, `apps/web/app/(auth)/login/page.tsx:9`
- Delete: nothing. No route, component or data hook is removed.

**Interfaces:**
- Produces: `NAV_GROUPS: NavGroup[]` and `NAV_ITEMS: NavItem[]` (`app/admin/nav.ts`); `<PageHeader title meta? actions?>`; `<StatusChip status>`; `<CommandPalette>` mounted once in the admin shell, opened by ⌘K / Ctrl+K or the top-bar pill; Tailwind utilities `font-heading`, `font-mono`, `bg-graphite`, `text-graphite-foreground`.
- Keeps (e2e depends on them): headings `Categories`, `New post`, `Edit post`; the `super_admin` role badge text in the top bar; every existing button label.

- [ ] **Step 1: Write `design.md`**

`design.md` (repo root):

````markdown
# Design — qdjr admin

Locked design system for `apps/web`. Every admin page reads this file before
changing UI. Amend this file to grow the system; never override per page.

## Genre
modern-minimal — instrument panel for one person who writes and publishes.

## Macrostructure family
- App pages: **Workbench**. Grouped left rail (240px, md+) · top bar (56px:
  ⌘K pill left, "New post" + user menu right) · content column `max-w-[1200px]`.
  List pages = PageHeader + one table. Editor pages = writing column + 300px
  sticky meta rail.
- Auth pages: centred single card on paper, wordmark above.
- No marketing or content pages live in this app.

## Theme — Cobalt
| Token (shadcn var) | Light | Dark |
|---|---|---|
| paper `--background` | oklch(98.5% 0.004 250) | oklch(18.5% 0.014 260) |
| surface `--card` / `--popover` | oklch(99.3% 0.003 250) | oklch(21.5% 0.016 260) |
| paper-2 `--muted` `--secondary` `--sidebar` | oklch(96.4% 0.006 252) | oklch(24.5% 0.016 260) |
| ink `--foreground` | oklch(24% 0.02 258) | oklch(94% 0.008 255) |
| ink-2 `--muted-foreground` | oklch(48% 0.018 257) | oklch(71% 0.016 256) |
| rule `--border` / `--input` | oklch(90.5% 0.008 255) | oklch(30% 0.016 260) |
| signal `--primary` / `--ring` | oklch(54% 0.20 258) | oklch(72% 0.15 256) |
| signal-ink `--primary-foreground` | oklch(98.5% 0.004 250) | oklch(18.5% 0.014 260) |
| wash `--accent` | oklch(95.5% 0.025 256) | oklch(28% 0.05 258) |
| wash-ink `--accent-foreground` | oklch(38% 0.15 258) | oklch(88% 0.06 256) |
| danger `--destructive` | oklch(55% 0.20 27) | oklch(68% 0.19 24) |
| graphite `--graphite` (code surfaces) | oklch(22% 0.016 260) | oklch(14.5% 0.012 260) |
| graphite-ink `--graphite-foreground` | oklch(92% 0.01 255) | oklch(92% 0.01 255) |

Signal covers under 5% of any screen: primary button, active rail tick,
focus ring, the published dot, link hover underline. Nothing else is blue.
No pure #fff or #000. No gradients, no glass, no shadows (hairlines carry depth).

## Typography
- Display: Space Grotesk 600, tracking -0.02em. Page titles `text-2xl`, desk greeting `text-3xl`. Always roman.
- Body: Inter 400/500, 14px base.
- Mono: JetBrains Mono 400/500 for slugs, dates, IDs, counts, kbd hints, and UPPERCASE labels (`text-[11px] tracking-[0.06em]`).

## Spacing
Tailwind 4pt scale only (`gap-2/3/4/6/8`, `p-4/6`). No arbitrary px except the
documented 240px rail, 300px meta rail, 1200px column.

## Radii
`--radius: 0.375rem` (6px) for buttons, inputs, menus. Code surfaces 10px (`rounded-[10px]`).

## Motion
- Easing `--ease-out: cubic-bezier(0.16, 1, 0.3, 1)`, `--dur-short: 160ms`.
- Allowed: dialog/palette fade, colour transitions on hover. No reveals, no bounce, no parallax.
- `prefers-reduced-motion: reduce` → no transforms (shadcn `animate-in` zoom disabled).

## Microinteractions stance
Silent success: toasts only for errors and for actions with no visible result.
Focus rings appear instantly, 2px signal at ≥3:1. Keyboard: ⌘K / Ctrl+K opens
the palette everywhere in `/admin`; ↑/↓ move, Enter runs, Esc closes.

## Copy
Sentence case. Buttons name the action ("Save", "Publish", "New post").
Counts and dates are real API values or `—`; never invented.

## Exports
Tokens live in `apps/web/app/globals.css` (`:root`, `.dark`, `@theme inline`).
````

- [ ] **Step 2: Tokens and fonts**

In `apps/web/app/globals.css`, FIND lines 9-12:

```css
  --font-sans: var(--font-sans);
  --font-mono: var(--font-geist-mono);
  --font-heading: var(--font-sans);
```
REPLACE with:
```css
  --font-sans: var(--font-inter);
  --font-mono: var(--font-jetbrains-mono);
  --font-heading: var(--font-space-grotesk);
  --color-graphite: var(--graphite);
  --color-graphite-foreground: var(--graphite-foreground);
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);
```

REPLACE the whole `:root { … }` block (lines 51-84) with:

```css
/* Hallmark · genre: modern-minimal · macrostructure: Workbench · theme: Cobalt
 * design-system: design.md · designed-as-app
 * pre-emit critique: P4 H4 E4 S4 R5 V4 */
:root {
  --radius: 0.375rem;
  --dur-short: 160ms;
  --background: oklch(98.5% 0.004 250);
  --foreground: oklch(24% 0.02 258);
  --card: oklch(99.3% 0.003 250);
  --card-foreground: oklch(24% 0.02 258);
  --popover: oklch(99.3% 0.003 250);
  --popover-foreground: oklch(24% 0.02 258);
  --primary: oklch(54% 0.20 258);
  --primary-foreground: oklch(98.5% 0.004 250);
  --secondary: oklch(96.4% 0.006 252);
  --secondary-foreground: oklch(24% 0.02 258);
  --muted: oklch(96.4% 0.006 252);
  --muted-foreground: oklch(48% 0.018 257);
  --accent: oklch(95.5% 0.025 256);
  --accent-foreground: oklch(38% 0.15 258);
  --destructive: oklch(55% 0.20 27);
  --border: oklch(90.5% 0.008 255);
  --input: oklch(90.5% 0.008 255);
  --ring: oklch(54% 0.20 258);
  --chart-1: oklch(54% 0.20 258);
  --chart-2: oklch(68% 0.12 256);
  --chart-3: oklch(80% 0.06 255);
  --chart-4: oklch(48% 0.018 257);
  --chart-5: oklch(24% 0.02 258);
  --sidebar: oklch(96.4% 0.006 252);
  --sidebar-foreground: oklch(24% 0.02 258);
  --sidebar-primary: oklch(54% 0.20 258);
  --sidebar-primary-foreground: oklch(98.5% 0.004 250);
  --sidebar-accent: oklch(95.5% 0.025 256);
  --sidebar-accent-foreground: oklch(38% 0.15 258);
  --sidebar-border: oklch(90.5% 0.008 255);
  --sidebar-ring: oklch(54% 0.20 258);
  --graphite: oklch(22% 0.016 260);
  --graphite-foreground: oklch(92% 0.01 255);
}
```

REPLACE the whole `.dark { … }` block (lines 86-118) with:

```css
.dark {
  --background: oklch(18.5% 0.014 260);
  --foreground: oklch(94% 0.008 255);
  --card: oklch(21.5% 0.016 260);
  --card-foreground: oklch(94% 0.008 255);
  --popover: oklch(21.5% 0.016 260);
  --popover-foreground: oklch(94% 0.008 255);
  --primary: oklch(72% 0.15 256);
  --primary-foreground: oklch(18.5% 0.014 260);
  --secondary: oklch(24.5% 0.016 260);
  --secondary-foreground: oklch(94% 0.008 255);
  --muted: oklch(24.5% 0.016 260);
  --muted-foreground: oklch(71% 0.016 256);
  --accent: oklch(28% 0.05 258);
  --accent-foreground: oklch(88% 0.06 256);
  --destructive: oklch(68% 0.19 24);
  --border: oklch(30% 0.016 260);
  --input: oklch(33% 0.016 260);
  --ring: oklch(72% 0.15 256);
  --chart-1: oklch(72% 0.15 256);
  --chart-2: oklch(60% 0.12 256);
  --chart-3: oklch(45% 0.06 256);
  --chart-4: oklch(71% 0.016 256);
  --chart-5: oklch(94% 0.008 255);
  --sidebar: oklch(16.5% 0.013 260);
  --sidebar-foreground: oklch(94% 0.008 255);
  --sidebar-primary: oklch(72% 0.15 256);
  --sidebar-primary-foreground: oklch(18.5% 0.014 260);
  --sidebar-accent: oklch(28% 0.05 258);
  --sidebar-accent-foreground: oklch(88% 0.06 256);
  --sidebar-border: oklch(30% 0.016 260);
  --sidebar-ring: oklch(72% 0.15 256);
  --graphite: oklch(14.5% 0.012 260);
  --graphite-foreground: oklch(92% 0.01 255);
}
```

FIND (end of `globals.css`, lines 120-130):

```css
@layer base {
  * {
    @apply border-border outline-ring/50;
  }
  body {
    @apply bg-background text-foreground;
  }
  html {
    @apply font-sans;
  }
}
```
REPLACE:
```css
@layer base {
  * {
    @apply border-border outline-ring/50;
  }
  body {
    @apply bg-background text-foreground;
  }
  html {
    @apply font-sans;
  }
  html, body {
    overflow-x: clip;
  }
  h1, h2, h3 {
    font-family: var(--font-heading);
    font-style: normal;
    letter-spacing: -0.02em;
  }
  :focus-visible {
    outline: 2px solid var(--ring);
    outline-offset: 2px;
  }
  @media (prefers-reduced-motion: reduce) {
    *, *::before, *::after {
      animation-duration: 1ms !important;
      transition-duration: 1ms !important;
    }
  }
}
```

Replace `apps/web/app/layout.tsx`:

```tsx
import type { Metadata } from 'next';
import { Inter, JetBrains_Mono, Space_Grotesk } from 'next/font/google';
import { Providers } from './providers';
import './globals.css';

const inter = Inter({ variable: '--font-inter', subsets: ['latin', 'vietnamese'] });
const spaceGrotesk = Space_Grotesk({
  variable: '--font-space-grotesk',
  subsets: ['latin', 'vietnamese'],
  weight: ['500', '600'],
});
const jetbrainsMono = JetBrains_Mono({
  variable: '--font-jetbrains-mono',
  subsets: ['latin', 'vietnamese'],
  weight: ['400', '500'],
});

export const metadata: Metadata = {
  title: 'qdjr admin',
  description: 'qdjr.dev admin',
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${inter.variable} ${spaceGrotesk.variable} ${jetbrainsMono.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col bg-background text-foreground">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
```

`suppressHydrationWarning` is required by next-themes, which sets the `class` on `<html>` before React hydrates.

In `apps/web/app/providers.tsx` FIND:

```tsx
import { Toaster } from '@/components/ui/sonner';
```
REPLACE:
```tsx
import { ThemeProvider } from 'next-themes';
import { Toaster } from '@/components/ui/sonner';
```
and FIND:
```tsx
    <QueryClientProvider client={client}>
      {children}
      <Toaster richColors position="top-right" />
    </QueryClientProvider>
```
REPLACE:
```tsx
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <QueryClientProvider client={client}>
        {children}
        <Toaster position="bottom-right" />
      </QueryClientProvider>
    </ThemeProvider>
```
(`richColors` dropped: it paints green/red toast floods, which break the one-signal rule.)

In `apps/web/app/(auth)/login/page.tsx:9` FIND `bg-zinc-50 px-4 py-12 dark:bg-black` → REPLACE `bg-background px-4 py-12`.

- [ ] **Step 3: Shared nav config, PageHeader, StatusChip**

`apps/web/app/admin/nav.ts`:

```ts
import {
  Folder,
  Image as ImageIcon,
  Inbox,
  LayoutDashboard,
  Newspaper,
  Settings,
  Tags,
  User as UserIcon,
  Users,
} from 'lucide-react';
import type { ComponentType, SVGProps } from 'react';

export interface NavItem {
  href: string;
  label: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

// One source for the rail and the ⌘K palette.
export const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Write',
    items: [
      { href: '/admin', label: 'Desk', icon: LayoutDashboard },
      { href: '/admin/posts', label: 'Posts', icon: Newspaper },
      { href: '/admin/media', label: 'Media', icon: ImageIcon },
    ],
  },
  {
    label: 'Organise',
    items: [
      { href: '/admin/categories', label: 'Categories', icon: Folder },
      { href: '/admin/tags', label: 'Tags', icon: Tags },
    ],
  },
  {
    label: 'Site',
    items: [
      { href: '/admin/profile', label: 'Profile', icon: UserIcon },
      { href: '/admin/settings', label: 'Settings', icon: Settings },
    ],
  },
  {
    label: 'People',
    items: [
      { href: '/admin/inbox', label: 'Inbox', icon: Inbox },
      { href: '/admin/users', label: 'Users', icon: Users },
    ],
  },
];

export const NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((group) => group.items);

export function isActive(pathname: string, href: string): boolean {
  if (href === '/admin') return pathname === '/admin';
  return pathname === href || pathname.startsWith(`${href}/`);
}
```

`apps/web/components/page-header.tsx`:

```tsx
import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: string;
  /** One mono line under the title: a count, a slug, a status. Real values only. */
  meta?: ReactNode;
  actions?: ReactNode;
}

export function PageHeader({ title, meta, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4 border-b pb-4">
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold [overflow-wrap:anywhere]">{title}</h1>
        {meta ? (
          <p className="mt-1 font-mono text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
            {meta}
          </p>
        ) : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </header>
  );
}
```

`apps/web/components/status-chip.tsx`:

```tsx
import { cn } from '@/lib/utils';

type Status = 'draft' | 'published' | 'archived';

const DOT: Record<Status, string> = {
  published: 'bg-primary',
  draft: 'border border-muted-foreground bg-transparent',
  archived: 'bg-muted-foreground/40',
};

export function StatusChip({ status }: { status: Status | undefined }) {
  const value = status ?? 'draft';
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-[0.06em]',
        value === 'archived' ? 'text-muted-foreground' : 'text-foreground'
      )}
    >
      <span aria-hidden="true" className={cn('size-1.5 rounded-full', DOT[value])} />
      {value}
    </span>
  );
}
```

- [ ] **Step 4: Rail + shell + ⌘K palette**

Replace `apps/web/app/admin/sidebar.tsx`:

```tsx
'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Menu } from 'lucide-react';
import { useState } from 'react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';

import { NAV_GROUPS, isActive } from './nav';

function Wordmark() {
  return (
    <Link href="/admin" className="font-heading text-[15px] font-semibold tracking-[-0.02em]">
      qdjr<span className="text-muted-foreground">/admin</span>
    </Link>
  );
}

function NavList({ pathname, onNavigate }: { pathname: string; onNavigate?: () => void }) {
  return (
    <nav aria-label="Admin" className="flex flex-col gap-6 px-3 py-4">
      {NAV_GROUPS.map((group) => (
        <div key={group.label} className="flex flex-col gap-0.5">
          <p className="px-3 pb-1 font-mono text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
            {group.label}
          </p>
          {group.items.map((item) => {
            const active = isActive(pathname, item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={onNavigate}
                aria-current={active ? 'page' : undefined}
                className={cn(
                  'relative flex h-8 items-center gap-2.5 whitespace-nowrap rounded-md px-3 text-sm transition-colors',
                  active
                    ? 'font-medium text-foreground before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary'
                    : 'text-muted-foreground hover:bg-background hover:text-foreground'
                )}
              >
                <Icon className="size-4 shrink-0" aria-hidden="true" />
                {item.label}
              </Link>
            );
          })}
        </div>
      ))}
    </nav>
  );
}

export function Sidebar() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <>
      <aside className="hidden md:sticky md:top-0 md:flex md:h-screen md:w-60 md:shrink-0 md:flex-col md:border-r md:bg-sidebar">
        <div className="flex h-14 items-center border-b px-6">
          <Wordmark />
        </div>
        <NavList pathname={pathname} />
      </aside>

      <div className="flex h-14 items-center gap-2 border-b bg-sidebar px-3 md:hidden">
        <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
          <SheetTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Open navigation" />}>
            <Menu className="size-4" />
          </SheetTrigger>
          <SheetContent side="left" className="w-64 bg-sidebar p-0">
            <SheetHeader className="h-14 justify-center border-b px-6">
              <SheetTitle className="font-heading">qdjr/admin</SheetTitle>
            </SheetHeader>
            <NavList pathname={pathname} onNavigate={() => setMobileOpen(false)} />
          </SheetContent>
        </Sheet>
        <Wordmark />
      </div>
    </>
  );
}
```

`apps/web/app/admin/command-palette.tsx`:

```tsx
'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useTheme } from 'next-themes';
import { Moon, PenLine, Search, Sun } from 'lucide-react';
import type { ComponentType, SVGProps } from 'react';

import { cn } from '@/lib/utils';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';

import { NAV_ITEMS } from './nav';

interface Command {
  id: string;
  label: string;
  hint: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  run: () => void;
}

export function useCommandPaletteHotkey(onOpen: () => void) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        onOpen();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onOpen]);
}

export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const router = useRouter();
  const { resolvedTheme, setTheme } = useTheme();
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);

  const commands = useMemo<Command[]>(() => {
    const go = (href: string) => () => router.push(href);
    return [
      { id: 'new-post', label: 'New post', hint: 'write', icon: PenLine, run: go('/admin/posts/new') },
      ...NAV_ITEMS.map((item) => ({ id: item.href, label: item.label, hint: 'go to', icon: item.icon, run: go(item.href) })),
      {
        id: 'theme',
        label: resolvedTheme === 'dark' ? 'Switch to light' : 'Switch to dark',
        hint: 'theme',
        icon: resolvedTheme === 'dark' ? Sun : Moon,
        run: () => setTheme(resolvedTheme === 'dark' ? 'light' : 'dark'),
      },
    ];
  }, [router, resolvedTheme, setTheme]);

  const results = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return needle ? commands.filter((c) => c.label.toLowerCase().includes(needle)) : commands;
  }, [commands, query]);

  const close = () => {
    onOpenChange(false);
    setQuery('');
    setActive(0);
  };

  const runAt = (index: number) => {
    const command = results[index];
    if (!command) return;
    close();
    command.run();
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActive((i) => Math.min(i + 1, results.length - 1));
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (event.key === 'Enter') {
      event.preventDefault();
      runAt(active);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => (next ? onOpenChange(true) : close())}>
      <DialogContent showCloseButton={false} className="top-[18vh] translate-y-0 gap-0 p-0 sm:max-w-lg">
        <DialogTitle className="sr-only">Command palette</DialogTitle>
        <div className="flex items-center gap-2 border-b px-3">
          <Search className="size-4 text-muted-foreground" aria-hidden="true" />
          <input
            autoFocus
            role="combobox"
            aria-expanded="true"
            aria-controls="palette-list"
            aria-activedescendant={results[active] ? `palette-${results[active].id}` : undefined}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={onKeyDown}
            placeholder="Type a command or page…"
            className="h-11 w-full bg-transparent font-mono text-sm outline-none placeholder:text-muted-foreground"
          />
        </div>
        <ul id="palette-list" role="listbox" className="max-h-80 overflow-y-auto p-1.5">
          {results.length === 0 ? (
            <li className="px-3 py-6 text-center text-sm text-muted-foreground">No match for “{query}”.</li>
          ) : (
            results.map((command, index) => {
              const Icon = command.icon;
              return (
                <li
                  key={command.id}
                  id={`palette-${command.id}`}
                  role="option"
                  aria-selected={index === active}
                  onMouseMove={() => setActive(index)}
                  onClick={() => runAt(index)}
                  className={cn(
                    'flex h-9 cursor-pointer items-center gap-2.5 rounded-md px-3 text-sm',
                    index === active ? 'bg-accent text-accent-foreground' : 'text-foreground'
                  )}
                >
                  <Icon className="size-4 shrink-0" aria-hidden="true" />
                  <span className="flex-1 truncate">{command.label}</span>
                  <span className="font-mono text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
                    {command.hint}
                  </span>
                </li>
              );
            })
          )}
        </ul>
      </DialogContent>
    </Dialog>
  );
}
```

Replace the return block of `apps/web/app/admin/layout.tsx` (lines 55-65) and its imports. FIND lines 3-10:

```tsx
import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';

import { getCurrentUser, onAuthStateChange, type AuthUser } from '@/lib/auth';
import { Skeleton } from '@/components/ui/skeleton';

import { Sidebar } from './sidebar';
import { UserMenu } from './user-menu';
```
REPLACE:
```tsx
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Plus, Search } from 'lucide-react';

import { getCurrentUser, onAuthStateChange, type AuthUser } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

import { CommandPalette, useCommandPaletteHotkey } from './command-palette';
import { Sidebar } from './sidebar';
import { UserMenu } from './user-menu';
```

FIND line 15 `  const [checking, setChecking] = useState(true);` → REPLACE:
```tsx
  const [checking, setChecking] = useState(true);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const openPalette = useCallback(() => setPaletteOpen(true), []);
  useCommandPaletteHotkey(openPalette);
```

FIND lines 55-65 (the final `return ( <div className="flex min-h-screen …"> … );`) → REPLACE:
```tsx
  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b bg-background/90 px-4 backdrop-blur-sm md:px-8">
          <button
            type="button"
            onClick={openPalette}
            className="flex h-8 min-w-0 max-w-sm flex-1 items-center gap-2 rounded-md border bg-card px-2.5 text-sm text-muted-foreground transition-colors hover:border-primary/50"
          >
            <Search className="size-4 shrink-0" aria-hidden="true" />
            <span className="truncate">Search or jump to…</span>
            <kbd className="ml-auto hidden rounded border px-1.5 font-mono text-[11px] sm:inline">⌘K</kbd>
          </button>
          <div className="ml-auto flex shrink-0 items-center gap-2">
            <Button size="sm" render={<Link href="/admin/posts/new" />} aria-label="New post">
              <Plus className="size-4" aria-hidden="true" />
              <span className="hidden sm:inline">New post</span>
            </Button>
            <UserMenu user={user} />
          </div>
        </header>
        <main className="mx-auto flex w-full max-w-[1200px] flex-1 flex-col gap-6 px-4 py-6 md:px-8 md:py-8">
          {children}
        </main>
      </div>
      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
    </div>
  );
```

FIND `apps/web/app/admin/layout.tsx:46`:
```tsx
      <div className="flex min-h-screen flex-col gap-3 p-6">
```
REPLACE (skeleton sits in the same column as the loaded page):
```tsx
      <div className="mx-auto flex min-h-screen w-full max-w-[1200px] flex-col gap-3 p-8">
```

- [ ] **Step 5: Theme switch in the user menu**

In `apps/web/app/admin/user-menu.tsx` FIND:
```tsx
import { LogOut } from 'lucide-react';
```
REPLACE:
```tsx
import { LogOut } from 'lucide-react';
import { useTheme } from 'next-themes';
```
FIND (in the dropdown-menu import list):
```tsx
  DropdownMenuLabel,
  DropdownMenuSeparator,
```
REPLACE:
```tsx
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
```
FIND:
```tsx
  const [signingOut, setSigningOut] = useState(false);
```
REPLACE:
```tsx
  const [signingOut, setSigningOut] = useState(false);
  const { theme = 'system', setTheme } = useTheme();
```
FIND:
```tsx
        <Badge variant="secondary">{me.role}</Badge>
```
REPLACE (keeps the exact `super_admin` text the login e2e asserts):
```tsx
        <Badge variant="outline" className="hidden font-mono text-[11px] uppercase tracking-[0.06em] sm:inline-flex">
          {me.role}
        </Badge>
```
FIND:
```tsx
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleSignOut} disabled={signingOut}>
```
REPLACE:
```tsx
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="font-mono text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
            Theme
          </DropdownMenuLabel>
          <DropdownMenuRadioGroup value={theme} onValueChange={(value) => setTheme(String(value))}>
            <DropdownMenuRadioItem value="system">System</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="light">Light</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="dark">Dark</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleSignOut} disabled={signingOut}>
```
Trap: the `super_admin` badge is now `hidden` below the `sm` breakpoint. The login e2e runs Desktop Chrome, so it still sees it; mobile users read the role inside the dropdown label.

- [ ] **Step 6: The Desk (dashboard)**

Replace `apps/web/app/admin/page.tsx`:

```tsx
'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowRight } from 'lucide-react';

import { apiList } from '@/lib/api';
import { timeAgo } from '@/lib/format';
import type { ContactMessage, Post } from '@/lib/types';
import { useMe } from '@/lib/me';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusChip } from '@/components/status-chip';

const DESK_SIZE = 5;

function usePosts(status: 'draft' | 'published') {
  return useQuery({
    queryKey: ['posts', 'desk', status],
    queryFn: () => apiList<Post>(`/v1/admin/posts?status=${status}&page=1&perPage=${DESK_SIZE}`),
  });
}

function Panel({ title, count, href, children }: { title: string; count?: number; href: string; children: React.ReactNode }) {
  return (
    <section className="flex min-w-0 flex-col rounded-[10px] border bg-card">
      <header className="flex items-center justify-between border-b px-4 py-3">
        <h2 className="text-sm font-semibold">
          {title}
          {count !== undefined ? <span className="ml-2 font-mono text-xs text-muted-foreground">{count}</span> : null}
        </h2>
        <Link href={href} className="inline-flex items-center gap-1 whitespace-nowrap text-xs text-muted-foreground hover:text-primary">
          View all <ArrowRight className="size-3" aria-hidden="true" />
        </Link>
      </header>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}

function Rows({ loading, empty, children }: { loading: boolean; empty: string; children: React.ReactNode[] }) {
  if (loading) {
    return (
      <div className="flex flex-col gap-2 p-4">
        <Skeleton className="h-4 w-2/3" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    );
  }
  if (children.length === 0) return <p className="px-4 py-6 text-sm text-muted-foreground">{empty}</p>;
  return <ul className="divide-y">{children}</ul>;
}

export default function DeskPage() {
  const me = useMe();
  const drafts = usePosts('draft');
  const published = usePosts('published');
  const inbox = useQuery({
    queryKey: ['contact-messages', 'desk'],
    queryFn: () => apiList<ContactMessage>(`/v1/admin/contact-messages?status=new&page=1&perPage=3`),
  });

  const draftRows = (drafts.data?.data ?? []).map((post) => (
    <li key={post.id}>
      <Link href={`/admin/posts/${post.id}`} className="flex items-center gap-3 px-4 py-3 hover:bg-muted">
        <span className="min-w-0 flex-1 truncate text-sm font-medium">{post.title || 'Untitled'}</span>
        <span className="hidden font-mono text-[11px] text-muted-foreground sm:inline">{timeAgo(post.updated_at)}</span>
      </Link>
    </li>
  ));

  const publishedRows = (published.data?.data ?? []).map((post) => (
    <li key={post.id}>
      <Link href={`/admin/posts/${post.id}`} className="flex items-center gap-3 px-4 py-3 hover:bg-muted">
        <StatusChip status="published" />
        <span className="min-w-0 flex-1 truncate text-sm">{post.title}</span>
        <span className="hidden font-mono text-[11px] text-muted-foreground sm:inline">/{post.slug}</span>
      </Link>
    </li>
  ));

  const inboxRows = (inbox.data?.data ?? []).map((message) => (
    <li key={message.id}>
      <Link href="/admin/inbox" className="flex flex-col gap-0.5 px-4 py-3 hover:bg-muted">
        <span className="flex items-baseline justify-between gap-3">
          <span className="truncate text-sm font-medium">{message.name}</span>
          <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{timeAgo(message.created_at)}</span>
        </span>
        <span className="truncate text-xs text-muted-foreground">{message.subject || message.body}</span>
      </Link>
    </li>
  ));

  return (
    <>
      <header className="flex flex-wrap items-end justify-between gap-4 border-b pb-6">
        <div className="min-w-0">
          <p className="font-mono text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
            {me.data?.role ?? '—'}
          </p>
          <h1 className="mt-1 text-3xl font-semibold">Desk</h1>
        </div>
        <Link
          href="/admin/posts/new"
          className="group inline-flex items-center gap-2 whitespace-nowrap text-sm font-medium text-foreground underline decoration-primary/40 underline-offset-4 hover:decoration-primary"
        >
          Write a new post <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
        </Link>
      </header>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <Panel title="Drafts in progress" count={drafts.data?.meta.total} href="/admin/posts">
          <Rows loading={drafts.isLoading} empty="No drafts. Everything is published or archived.">{draftRows}</Rows>
        </Panel>
        <Panel title="New in inbox" count={inbox.data?.meta.total} href="/admin/inbox">
          <Rows loading={inbox.isLoading} empty="Inbox zero.">{inboxRows}</Rows>
        </Panel>
      </div>

      <Panel title="Recently published" count={published.data?.meta.total} href="/admin/posts">
        <Rows loading={published.isLoading} empty="Nothing published yet.">{publishedRows}</Rows>
      </Panel>
    </>
  );
}
```

Relative times reuse the existing `timeAgo(iso: string)` in `apps/web/lib/format.ts`; no new helper.

Dashboard data is honest: every count is `meta.total` from the API, absent counts render nothing, empty lists say so. The old "Permissions" badge wall is dropped from the Desk (permissions still drive the API; the role sits in the eyebrow and the top-bar badge).

- [ ] **Step 7: List pages and the editor**

Replace each list page with its full new content (the page wrapper's `flex flex-col gap-4` goes because the shell's `<main>` already stacks children with `gap-6`).
`apps/web/app/admin/categories/page.tsx`:

```tsx
import { PageHeader } from '@/components/page-header';

import { CategoriesTable } from './categories-table';

export default function CategoriesListPage() {
  return (
    <>
      <PageHeader title="Categories" />
      <CategoriesTable />
    </>
  );
}
```
`apps/web/app/admin/tags/page.tsx`:

```tsx
import { PageHeader } from '@/components/page-header';

import { TagsTable } from './tags-table';

export default function TagsListPage() {
  return (
    <>
      <PageHeader title="Tags" />
      <TagsTable />
    </>
  );
}
```
`apps/web/app/admin/profile/page.tsx`:

```tsx
import { PageHeader } from '@/components/page-header';

import { ProfileForm } from './profile-form';

export default function ProfilePage() {
  return (
    <>
      <PageHeader title="Profile" />
      <ProfileForm />
    </>
  );
}
```
`apps/web/app/admin/settings/page.tsx`:

```tsx
import { PageHeader } from '@/components/page-header';

import { SettingsForm } from './settings-form';

export default function SettingsPage() {
  return (
    <>
      <PageHeader title="Site settings" />
      <SettingsForm />
    </>
  );
}
```
`apps/web/app/admin/inbox/page.tsx`:

```tsx
import { PageHeader } from '@/components/page-header';

import { Inbox } from './inbox';

export default function InboxPage() {
  return (
    <>
      <PageHeader title="Contact inbox" />
      <Inbox />
    </>
  );
}
```
`apps/web/app/admin/users/page.tsx`:

```tsx
import { PageHeader } from '@/components/page-header';

import { UsersTable } from './users-table';

export default function UsersPage() {
  return (
    <>
      <PageHeader title="Users" />
      <UsersTable />
    </>
  );
}
```
`apps/web/app/admin/media/page.tsx`:

```tsx
import { PageHeader } from '@/components/page-header';

import { MediaGallery } from './media-gallery';
import { UploadDropzone } from './upload-dropzone';

export default function MediaPage() {
  return (
    <>
      <PageHeader title="Media" />
      <UploadDropzone />
      <MediaGallery />
    </>
  );
}
```

Verify: `grep -rn 'text-2xl font-semibold' apps/web/app/admin` → only `posts/post-form.tsx:169` remains until the editor edit below.

`apps/web/app/admin/posts/page.tsx` → replace with:

```tsx
import Link from 'next/link';

import { Button } from '@/components/ui/button';
import { PageHeader } from '@/components/page-header';

import { PostsTable } from './posts-table';

export default function PostsListPage() {
  return (
    <>
      <PageHeader title="Posts" actions={<Button render={<Link href="/admin/posts/new" />}>New post</Button>} />
      <PostsTable />
    </>
  );
}
```

`apps/web/app/admin/posts/posts-table.tsx` (`Badge` stays imported: the category cell still uses it).

FIND lines 13-14:
```tsx
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
```
REPLACE:
```tsx
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusChip } from '@/components/status-chip';
```
FIND lines 50-57:
```tsx
function statusBadgeVariant(
  status: Post['status']
): 'default' | 'secondary' | 'outline' {
  if (status === 'published') return 'default';
  if (status === 'archived') return 'outline';
  return 'secondary';
}

```
REPLACE with nothing (delete the function and the blank line after it).
FIND lines 148:
```tsx
      <div className="rounded-lg border">
```
REPLACE:
```tsx
      <div className="overflow-x-auto rounded-[10px] border bg-card">
```
FIND lines 184:
```tsx
                <TableCell colSpan={6} className="text-center text-red-600">
```
REPLACE:
```tsx
                <TableCell colSpan={6} className="text-center text-destructive">
```
FIND lines 208-215:
```tsx
                  <TableCell className="text-muted-foreground">
                    {post.slug}
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusBadgeVariant(post.status)}>
                      {post.status ?? 'draft'}
                    </Badge>
                  </TableCell>
```
REPLACE:
```tsx
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    /{post.slug}
                  </TableCell>
                  <TableCell>
                    <StatusChip status={post.status} />
                  </TableCell>
```
FIND lines 223-225:
```tsx
                  <TableCell className="text-muted-foreground">
                    {formatDate(post.published_at)}
                  </TableCell>
```
REPLACE:
```tsx
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {formatDate(post.published_at)}
                  </TableCell>
```

`post-form.tsx` — FIND lines 167-185:

```tsx
    <form onSubmit={onSubmit} className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">
          {isEdit ? 'Edit post' : 'New post'}
        </h1>
        <div className="flex items-center gap-2">
          {isEdit && initial ? (
            <PostActions post={initial} />
          ) : null}
          <Button type="submit" disabled={submitting}>
            {submitting ? 'Saving…' : isEdit ? 'Save' : 'Create'}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <div className="flex flex-col gap-4 md:col-span-2">
```
REPLACE:
```tsx
    <form onSubmit={onSubmit} className="flex flex-col gap-6">
      <PageHeader
        title={isEdit ? 'Edit post' : 'New post'}
        meta={isEdit && initial ? <StatusChip status={initial.status} /> : 'unsaved'}
        actions={
          <>
            {isEdit && initial ? <PostActions post={initial} /> : null}
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Saving…' : isEdit ? 'Save' : 'Create'}
            </Button>
          </>
        }
      />

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_300px]">
        <div className="flex min-w-0 flex-col gap-5">
```

FIND the right-column opener (line ~236, the first `<div className="flex flex-col gap-4">` after the content `</FormField>`):
```tsx
        <div className="flex flex-col gap-4">
          <FormField label="Status" error={errors.status?.message}>
```
REPLACE:
```tsx
        <aside className="flex flex-col gap-4 lg:sticky lg:top-20 lg:self-start lg:border-l lg:pl-6">
          <FormField label="Status" error={errors.status?.message}>
```
and FIND lines 350-353:
```tsx
          </FormField>
        </div>
      </div>
    </form>
```
REPLACE:
```tsx
          </FormField>
        </aside>
      </div>
    </form>
```

FIND (Title input):
```tsx
              placeholder="Post title"
            />
```
REPLACE:
```tsx
              placeholder="Post title"
              className="h-12 font-heading text-xl font-semibold"
            />
```
FIND (Slug input):
```tsx
              placeholder="my-post-slug"
            />
```
REPLACE:
```tsx
              placeholder="my-post-slug"
              className="font-mono text-sm"
            />
```
FIND (Content textarea):
```tsx
              rows={15}
              placeholder="Write your post…"
              className="font-mono"
```
REPLACE:
```tsx
              rows={24}
              placeholder="Write your post…"
              className="min-h-[28rem] font-mono text-[13px] leading-6"
```
FIND lines 34-36:
```tsx
import { MediaPicker } from '@/components/media-picker';

import { PostActions } from './post-actions';
```
REPLACE:
```tsx
import { MediaPicker } from '@/components/media-picker';
import { PageHeader } from '@/components/page-header';
import { StatusChip } from '@/components/status-chip';

import { PostActions } from './post-actions';
```

The `h1` text stays "New post" / "Edit post", so `post-lifecycle.spec.ts` keeps matching.

`apps/web/app/admin/profile/profile-form.tsx` (the JSON editor added in Task 5) — FIND:
```tsx
          spellCheck={false}
          className="font-mono text-xs"
```
REPLACE:
```tsx
          spellCheck={false}
          className="rounded-[10px] border-0 bg-graphite font-mono text-xs leading-5 text-graphite-foreground"
```
This is the theme's one graphite surface: code reads as code. Also FIND in the same file `<p className="text-sm text-red-600">` → REPLACE `<p className="text-sm text-destructive">`.

- [ ] **Step 8: Typecheck, lint, e2e**

```bash
cd apps/web && pnpm exec tsc --noEmit && pnpm lint && pnpm build 2>&1 | tail -3
E2E_API_BASE=http://localhost:8080 pnpm exec playwright test 2>&1 | tail -4
grep -rnE "oklch\(|#[0-9a-fA-F]{3,6}\b|zinc-|slate-|gray-|red-600" app components --include=*.tsx | grep -v "components/ui/" ; echo "raw-colour-exit=$?"
```
Expected: no type or lint errors; `4 passed`; `raw-colour-exit=1` (no colour values outside the token block, Hallmark gate 48). The 13 existing `text-red-600` error lines count as hits: replace each with `text-destructive` (`grep -rln text-red-600 app components | xargs sed -i '' 's/text-red-600/text-destructive/g'`).

- [ ] **Step 9: Slop test + responsive check**

With the stack running (Task 12 step 1), open `/admin`, `/admin/posts`, `/admin/posts/new`, `/admin/profile`, `/login` in the browser at 320, 375, 414, 768 and 1280px, light and dark (toggle from the user menu). Then load `~/.claude/skills/hallmark/references/slop-test.md` and run every universal gate plus the modern-minimal overrides. Hard checks:
- no horizontal scroll at 320px (`document.documentElement.scrollWidth === innerWidth`),
- no button, rail link or top-bar control wraps to two lines,
- ⌘K opens centred near the top, ↑/↓/Enter/Esc work, focus returns to the pill,
- signal blue covers under 5% of each screen (only primary button, active tick, focus ring, published dots),
- headings upright Space Grotesk, body Inter, slugs and dates JetBrains Mono (check computed `font-family`),
- dark mode: every surface uses the `.dark` tokens, no white flashes on load.
Record fails, fix, rerun. Update the critique stamp at the top of `:root` if any axis changes.

- [ ] **Step 10: Project memory + commit**

`.hallmark/log.json`:

```json
[
  {
    "date": "2026-10-12",
    "scope": "app",
    "genre": "modern-minimal",
    "macrostructure": "Workbench",
    "theme": "Cobalt",
    "nav": "grouped side rail + N13 inline ⌘K pill",
    "footer": "none",
    "enrichment": "none",
    "brief": "qdjr admin · single-owner writing desk"
  }
]
```

```bash
git add design.md .hallmark/log.json apps/web/app apps/web/components
git commit -m "feat(web): Cobalt admin redesign — Desk dashboard, grouped rail, ⌘K palette, dark mode"
```

**Done when:**
- [ ] `design.md` and `.hallmark/log.json` exist with the content in steps 1 and 10
- [ ] computed `font-family` of `h1` is Space Grotesk, body Inter, slug cells JetBrains Mono
- [ ] ⌘K / Ctrl+K opens the palette on every `/admin` page; ↑/↓/Enter/Esc work
- [ ] theme switch in the user menu toggles light/dark/system and persists across reload
- [ ] no horizontal scroll at 320px; no wrapped button or rail labels at 320/375/414/768px
- [ ] raw-colour grep in step 8 prints nothing; all 4 web e2e specs pass
- [ ] no route, query key or e2e-visible label removed (`git diff --stat main -- apps/web/app` shows modifications only, no deletions)

---

### Task 6: `$api` client, profile and settings from the API

**Files:**
- Replace: `apps/site/app/plugins/api.ts`
- Create: `apps/site/app/utils/profileContent.ts`, `apps/site/app/composables/useSiteSettings.ts`
- Replace: `apps/site/app/composables/useProfile.ts`
- Modify: `apps/site/app/app.vue:7-16`, `apps/site/nuxt.config.ts:32-36`
- Test: `apps/site/test/profileContent.spec.ts`

**Interfaces:**
- Produces: `$api.listPosts(filter) → {posts: ApiPost[]; total}`, `$api.getPost(slug) → ApiPost | null` (404 → null), `$api.listCategories()`, `$api.listTags()`, `$api.getProfile() → ApiProfile | null`, `$api.getSiteSettings() → ApiSiteSettings | null`; types `ApiPost`, `ApiCategory`, `ApiTag`, `ApiProfile`, `ApiSiteSettings`, `PostFilter`; `pickProfileContent(content, locale) → ProfileContent | null`; `useSiteShell()` (the `site-shell` payload `{profile, settings}`), `useProfile()` (unchanged signature: `ComputedRef<ProfileContent>`), `useSiteSettings() → ComputedRef<ApiSiteSettings | null>`.
- Note for Tasks 7/8: the old `$api` (legacy contract) and the Pinia stores that used it are deleted in Task 8; until then `test/stores.spec.ts` fails and `typeCheck: false` in `nuxt.config.ts` keeps the build green.

- [ ] **Step 1: Failing unit test for the profile adapter**

`apps/site/test/profileContent.spec.ts`:

```ts
import {describe, expect, test} from 'vitest'
import {PROFILE_CONTENT} from '~/data/profile'
import {pickProfileContent} from '~/utils/profileContent'

describe('pickProfileContent', () => {
  test('returns the locale entry when it looks like ProfileContent', () => {
    const content = {vi: PROFILE_CONTENT.vi, en: PROFILE_CONTENT.en}
    expect(pickProfileContent(content, 'en')).toBe(content.en)
  })

  test('returns null for missing, non-object or incomplete content', () => {
    expect(pickProfileContent(undefined, 'vi')).toBeNull()
    expect(pickProfileContent(null, 'vi')).toBeNull()
    expect(pickProfileContent({}, 'vi')).toBeNull()
    expect(pickProfileContent({vi: {profile: {name: 'x'}}}, 'vi')).toBeNull()
    expect(pickProfileContent({vi: 'nope'}, 'vi')).toBeNull()
  })
})
```

```bash
cd apps/site && npx vitest run test/profileContent.spec.ts 2>&1 | tail -4
```
Expected: FAIL `Failed to resolve import "~/utils/profileContent"`.

- [ ] **Step 2: Adapter**

`apps/site/app/utils/profileContent.ts`:

```ts
import type {ProfileContent} from '~/data/profile'

const SECTIONS: ReadonlyArray<keyof ProfileContent> = ['profile', 'quote', 'skills', 'otherSkills', 'timeline', 'projects', 'contact']

/**
 * The ProfileContent for a locale out of the API's opaque `profile.content`
 * blob ({"vi": …, "en": …}), or null when it is missing or not shaped like one.
 * Callers fall back to the bundled copy on null, so a half-edited blob in the
 * admin can never blank the home page.
 */
export function pickProfileContent(content: unknown, locale: string): ProfileContent | null {
  if (!content || typeof content !== 'object') return null
  const entry = (content as Record<string, unknown>)[locale]
  if (!entry || typeof entry !== 'object') return null
  const hasAll = SECTIONS.every((key) => key in (entry as Record<string, unknown>))
  return hasAll ? (entry as ProfileContent) : null
}
```

```bash
cd apps/site && npx vitest run test/profileContent.spec.ts 2>&1 | tail -4
```
Expected: `2 passed`.

- [ ] **Step 3: Replace the plugin**

`apps/site/app/plugins/api.ts` (whole file):

```ts
// $api: the qdjr-admin public API (apps/api, /v1/*). Server- and client-side.

export interface ApiCategory {
  id: string
  slug: string
  name: string
  description: string | null
  created_at: string
  updated_at: string
}

export type ApiTag = ApiCategory

export interface ApiPost {
  id: string
  slug: string
  title: string
  excerpt: string | null
  content: string // markdown
  published_at: string | null
  thumbnail: {url: string; alt: string} | null
  location: string | null
  category: Pick<ApiCategory, 'id' | 'slug' | 'name'> | null
  tags: Array<Pick<ApiTag, 'id' | 'slug' | 'name'>>
  created_at: string
  updated_at: string
}

export interface ApiProfile {
  id: number
  full_name: string | null
  bio: string | null
  avatar_url: string | null
  tagline: string | null
  social_links: Record<string, string>
  location: string | null
  email: string | null
  content: Record<string, unknown>
  updated_at: string
}

export interface ApiSiteSettings {
  id: number
  site_title: string
  site_description: string | null
  footer_text: string | null
  contact_email: string | null
  social_links: Record<string, string>
  updated_at: string
}

export interface PostFilter {
  page?: number
  perPage?: number
  category?: string
  tag?: string
  q?: string
}

interface Envelope<T> {
  data: T | null
  meta?: {page: number; perPage: number; total: number}
  error: {code: string; message: string} | null
}

const isNotFound = (error: unknown) => {
  const e = error as {status?: number; statusCode?: number} | null
  return e?.status === 404 || e?.statusCode === 404
}

export default defineNuxtPlugin(() => {
  const base = `${String(useRuntimeConfig().public.apiUrl).replace(/\/+$/, '')}/v1`
  // Fail fast: pages fall back (empty list, bundled profile) rather than hang
  // a Netlify function on a sleeping API.
  const opts = {timeout: 8000, retry: 0 as const}

  const get = <T>(path: string, query?: Record<string, string | number | undefined>) =>
    $fetch<Envelope<T>>(`${base}${path}`, {...opts, query})

  const api = {
    async listPosts(filter: PostFilter = {}): Promise<{posts: ApiPost[]; total: number}> {
      const res = await get<ApiPost[]>('/posts', {
        page: filter.page ?? 1,
        perPage: filter.perPage ?? 50,
        category: filter.category || undefined,
        tag: filter.tag || undefined,
        q: filter.q || undefined
      })
      return {posts: res.data ?? [], total: res.meta?.total ?? 0}
    },

    async getPost(slug: string): Promise<ApiPost | null> {
      try {
        return (await get<ApiPost>(`/posts/${encodeURIComponent(slug)}`)).data
      } catch (error) {
        if (isNotFound(error)) return null
        throw error
      }
    },

    async listCategories(): Promise<ApiCategory[]> {
      return (await get<ApiCategory[]>('/categories')).data ?? []
    },

    async listTags(): Promise<ApiTag[]> {
      return (await get<ApiTag[]>('/tags')).data ?? []
    },

    async getProfile(): Promise<ApiProfile | null> {
      return (await get<ApiProfile>('/profile')).data
    },

    async getSiteSettings(): Promise<ApiSiteSettings | null> {
      return (await get<ApiSiteSettings>('/site-settings')).data
    }
  }

  return {provide: {api}}
})
```

- [ ] **Step 4: Runtime config**

FIND `apps/site/nuxt.config.ts:32-36`:

```ts
      // Empty by default: api.qdjr.me no longer resolves (NXDOMAIN). An empty
      // value makes every $api call short-circuit to an empty result with zero
      // network traffic, instead of a multi-second DNS failure on each
      // navigation. Set API_URL to re-enable the legacy blog surface.
      apiUrl: process.env.API_URL || '',
```

REPLACE with:

```ts
      // qdjr-admin API (apps/api). Netlify sets API_URL; local dev uses the Go
      // server on :8080.
      apiUrl: process.env.API_URL || 'https://api.qdjr.dev',
```

- [ ] **Step 5: Shell loader, `useSiteShell`, `useProfile`, `useSiteSettings`**

`apps/site/app/composables/useSiteSettings.ts`:

```ts
import type {ApiProfile, ApiSiteSettings} from '~/plugins/api'

export interface SiteShell {
  profile: ApiProfile | null
  settings: ApiSiteSettings | null
}

/** The profile + site settings app.vue fetched once for this request (null parts when the API was down). */
export function useSiteShell() {
  return useNuxtData<SiteShell>('site-shell').data
}

export function useSiteSettings() {
  const shell = useSiteShell()
  return computed(() => shell.value?.settings ?? null)
}
```

`apps/site/app/composables/useProfile.ts` (whole file):

```ts
import {PROFILE_CONTENT, type ProfileLocale} from '~/data/profile'
import {pickProfileContent} from '~/utils/profileContent'

/**
 * Profile copy for the current page language: the API's profile.content[locale]
 * (edited in qdjr-admin → Profile → Site content), else the bundled copy in
 * ~/data/profile.* so the site still renders while the API is down.
 */
export function useProfile() {
  const {locale} = useI18n()
  const shell = useSiteShell()
  return computed(
    () =>
      pickProfileContent(shell.value?.profile?.content, locale.value) ??
      PROFILE_CONTENT[locale.value as ProfileLocale] ??
      PROFILE_CONTENT.vi
  )
}
```

FIND `apps/site/app/app.vue:7-16`:

```ts
<script setup lang="ts">
// <html lang>, plus hreflang alternates for both languages on every page.
const i18nHead = useLocaleHead({seo: true})

useHead(() => ({
  htmlAttrs: {lang: i18nHead.value.htmlAttrs?.lang},
  link: [...(i18nHead.value.link || [])],
  meta: [...(i18nHead.value.meta || [])]
}))
</script>
```

REPLACE with:

```ts
<script setup lang="ts">
import type {SiteShell} from '~/composables/useSiteSettings'

// <html lang>, plus hreflang alternates for both languages on every page.
const i18nHead = useLocaleHead({seo: true})

useHead(() => ({
  htmlAttrs: {lang: i18nHead.value.htmlAttrs?.lang},
  link: [...(i18nHead.value.link || [])],
  meta: [...(i18nHead.value.meta || [])]
}))

// Profile + site settings once per request; every page reads them through
// useProfile() / useSiteSettings() via useNuxtData('site-shell'). Each part
// degrades to null on its own so one failing call never blanks the page.
const {$api} = useNuxtApp()
const quiet = async <T>(label: string, load: () => Promise<T | null>): Promise<T | null> => {
  try {
    return await load()
  } catch (error) {
    console.error(`[site] ${label} failed:`, (error as Error).message)
    return null
  }
}
await useAsyncData<SiteShell>('site-shell', async () => {
  const [profile, settings] = await Promise.all([
    quiet('profile', () => $api.getProfile()),
    quiet('site-settings', () => $api.getSiteSettings())
  ])
  return {profile, settings}
})
</script>
```

- [ ] **Step 6: Smoke in dev against the local API**

(API on :8080 with the seeded DB — Task 12 step 1.)
```bash
cd apps/site && API_URL=http://localhost:8080 npx nuxt dev -p 3001 >/tmp/site-dev.log 2>&1 &
sleep 12; curl -s http://localhost:3001/en/about | grep -o 'Senior Backend Software Engineer' | head -1; grep -c 'profile failed' /tmp/site-dev.log
pkill -f 'nuxt dev -p 3001'
```
Expected: `Senior Backend Software Engineer` and `0` (no fallback was needed).

- [ ] **Step 7: Commit**

```bash
cd apps/site && npm test 2>&1 | tail -3; cd ../..
git add apps/site/app/plugins/api.ts apps/site/app/utils/profileContent.ts apps/site/app/composables/useSiteSettings.ts apps/site/app/composables/useProfile.ts apps/site/app/app.vue apps/site/nuxt.config.ts apps/site/test/profileContent.spec.ts
git commit -m "feat(site): \$api client; profile and settings load from qdjr-admin"
```
(Expect `test/stores.spec.ts` to fail here because the old `$api` contract is gone; it is deleted in Task 8. Do not fix it.)

**Done when:**
- [ ] `test/profileContent.spec.ts` passes
- [ ] `/en/about` renders from the API (dev log has no `[site] profile failed`)
- [ ] with the API stopped, `/` and `/about` still render the bundled copy

---

### Task 7: Blog pages read posts from the API; drop `@nuxt/content`

**Files:**
- Create: `apps/site/app/utils/markdown.ts`, `apps/site/test/markdown.spec.ts`, `apps/site/test/blogPosts.spec.ts`
- Replace: `apps/site/app/composables/useBlogPosts.ts`
- Modify: `apps/site/app/pages/blog/[slug].vue:33, 51-59, 70-82, 107, 117-159`, `blog/category/[category].vue` script, `blog/tag/[tag].vue` script
- Delete: `apps/site/content.config.ts`, `apps/site/content/`, `apps/site/scripts/translate-posts.mjs`, `apps/site/scripts/translate/`, `apps/site/test/translateMarkdown.spec.ts`, `apps/site/app/utils/blogLocale.ts`, `apps/site/test/blogLocale.spec.ts`
- Modify: `apps/site/nuxt.config.ts:119-141`, `apps/site/package.json`

**Interfaces:**
- Consumes: `$api.listPosts`, `$api.getPost`, `ApiPost` (Task 6).
- Produces: `BlogPost {path, slug, title, description, publishedAt, updatedAt, thumbnail?, cover?, category?, tags: string[], html?, originalOnly?}`; `toBlogPost(post: ApiPost, locale: string, withBody = false): BlogPost`; `fetchBlogPosts(locale, filter?: {category?, tag?}): Promise<BlogPost[]>` (never throws); `fetchBlogPost(slug, locale): Promise<BlogPost | null>`; `renderMarkdown(source: string): string`.

- [ ] **Step 1: Failing tests**

`apps/site/test/markdown.spec.ts`:

```ts
import {describe, expect, test} from 'vitest'
import {renderMarkdown} from '~/utils/markdown'

describe('renderMarkdown', () => {
  test('renders headings, links and fenced code', () => {
    const html = renderMarkdown('# Hi\n\n[x](https://qdjr.dev)\n\n```ts\nconst a = 1\n```\n')
    expect(html).toContain('<h1>Hi</h1>')
    expect(html).toContain('<a href="https://qdjr.dev">x</a>')
    expect(html).toContain('<pre><code class="language-ts">')
  })

  test('keeps raw HTML written by the admin and tolerates empty input', () => {
    expect(renderMarkdown('<video src="a.mp4"></video>')).toContain('<video src="a.mp4">')
    expect(renderMarkdown('')).toBe('')
  })
})
```

`apps/site/test/blogPosts.spec.ts`:

```ts
import {describe, expect, test} from 'vitest'
import {setMockApi} from './setup'
import {fetchBlogPosts, toBlogPost} from '~/composables/useBlogPosts'
import type {ApiPost} from '~/plugins/api'

const post: ApiPost = {
  id: '1',
  slug: 'hello-world',
  title: 'Hello',
  excerpt: 'desc',
  content: '# Body',
  published_at: '2026-04-19T02:00:00Z',
  thumbnail: null,
  location: null,
  category: null,
  tags: [{id: 't', slug: 'nuxt', name: 'Nuxt'}],
  created_at: '2026-04-19T02:00:00Z',
  updated_at: '2026-04-19T03:00:00Z'
}

describe('toBlogPost', () => {
  test('maps API fields to the page shape and copes with no category/thumbnail', () => {
    const vi = toBlogPost(post, 'vi')
    expect(vi).toMatchObject({path: '/blog/hello-world', title: 'Hello', description: 'desc', tags: ['nuxt']})
    expect(vi.category).toBeUndefined()
    expect(vi.thumbnail).toBeUndefined()
    expect(vi.html).toBeUndefined()
    expect(vi.originalOnly).toBeUndefined()
  })

  test('flags English pages as original-only and renders the body on demand', () => {
    const en = toBlogPost({...post, category: {id: 'c', slug: 'meta', name: 'Meta'}}, 'en', true)
    expect(en.originalOnly).toBe(true)
    expect(en.category).toBe('meta')
    expect(en.html).toContain('<h1>Body</h1>')
  })
})

describe('fetchBlogPosts', () => {
  test('passes the filter through and maps the list', async () => {
    const calls: unknown[] = []
    setMockApi({listPosts: async (f: unknown) => (calls.push(f), {posts: [post], total: 1})})
    const list = await fetchBlogPosts('vi', {tag: 'nuxt'})
    expect(list.map((p) => p.slug)).toEqual(['hello-world'])
    expect(calls[0]).toMatchObject({tag: 'nuxt'})
  })

  test('returns an empty list when the API is down', async () => {
    setMockApi({listPosts: async () => { throw new Error('ECONNREFUSED') }})
    expect(await fetchBlogPosts('vi')).toEqual([])
  })
})
```

```bash
cd apps/site && npx vitest run test/markdown.spec.ts test/blogPosts.spec.ts 2>&1 | tail -5
```
Expected: FAIL (modules not found).

- [ ] **Step 2: Dependencies**

```bash
cd apps/site && npm uninstall @nuxt/content better-sqlite3 && npm install markdown-it@^14 && npm install -D @types/markdown-it@^14 && cd ../..
```
Then in `apps/site/package.json` FIND:
```json
    "translate:posts": "node scripts/translate-posts.mjs",
    "trips:build": "node scripts/build-trip.mjs"
```
REPLACE:
```json
    "trips:build": "node scripts/build-trip.mjs"
```

- [ ] **Step 3: Markdown + composable**

`apps/site/app/utils/markdown.ts`:

```ts
import MarkdownIt from 'markdown-it'

// ponytail: no HTML sanitizer. Post bodies are written by authenticated admins
// in qdjr-admin, never by visitors; add DOMPurify/rehype-sanitize if authors
// ever become untrusted.
const md = new MarkdownIt({html: true, linkify: true})

/** Markdown (as stored in posts.content) → HTML for v-html. */
export function renderMarkdown(source: string): string {
  return md.render(source ?? '')
}
```

`apps/site/app/composables/useBlogPosts.ts` (whole file):

```ts
import type {ApiPost} from '~/plugins/api'
import {renderMarkdown} from '~/utils/markdown'

/** What the blog pages render. Posts are written in Vietnamese; the English site shows them as-is and says so. */
export interface BlogPost {
  path: string
  slug: string
  title: string
  description: string
  publishedAt: string | null
  updatedAt: string
  thumbnail?: string
  cover?: string
  category?: string
  tags: string[]
  html?: string
  originalOnly?: boolean
}

export function toBlogPost(post: ApiPost, locale: string, withBody = false): BlogPost {
  return {
    path: `/blog/${post.slug}`,
    slug: post.slug,
    title: post.title,
    description: post.excerpt ?? '',
    publishedAt: post.published_at,
    updatedAt: post.updated_at,
    thumbnail: post.thumbnail?.url,
    cover: post.thumbnail?.url,
    category: post.category?.slug,
    tags: post.tags.map((tag) => tag.slug),
    html: withBody ? renderMarkdown(post.content) : undefined,
    originalOnly: locale !== 'vi' ? true : undefined
  }
}

/** Published posts, newest first, optionally narrowed by category/tag slug. Empty when the API is unreachable. */
export async function fetchBlogPosts(locale: string, filter: {category?: string; tag?: string} = {}): Promise<BlogPost[]> {
  const {$api} = useNuxtApp()
  try {
    const {posts} = await $api.listPosts({...filter, perPage: 100})
    return posts.map((post) => toBlogPost(post, locale))
  } catch (error) {
    console.error('[blog] list failed:', (error as Error).message)
    return []
  }
}

/** One post with its rendered body; null when it does not exist (404). Other failures propagate to the Nuxt error page. */
export async function fetchBlogPost(slug: string, locale: string): Promise<BlogPost | null> {
  const {$api} = useNuxtApp()
  const post = await $api.getPost(slug)
  return post ? toBlogPost(post, locale, true) : null
}
```

```bash
cd apps/site && npx vitest run test/markdown.spec.ts test/blogPosts.spec.ts 2>&1 | tail -4
```
Expected: `6 passed`.

- [ ] **Step 4: Pages**

Replace `apps/site/app/pages/blog/[slug].vue` entirely (the `ContentRenderer`, the author card, the locale-path lookup and the Shiki `<style>` overrides go; everything else is unchanged):

```vue
<template>
  <div class="h-full">
    <HeaderContent v-if="post" :title="post.title" :background="post.cover || ''" :lang="articleLang"/>

    <div v-if="post"
         class="flex pb-8 px-1 sm:px-2 my-12 border-b md:col-span-3 border-gray-200 dark:border-gray-600 w-full max-w-(--breakpoint-lg) mx-auto">
      <div class="w-full grow text-base">
        <div class="w-full sm:px-3 text-gray-800 dark:text-gray-200 leading-normal">
          <PostTranslationNotice :post="post"/>

          <div class="md:flex md:justify-between items-center mb-8 font-bold">
            <span v-if="post.category" class="flex items-center text-right text-gray-700 dark:text-gray-200">
              <NuxtLinkLocale :to="`/blog/category/${post.category}`" class="flex items-center">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"/>
                </svg>
                <span>{{ post.category }}</span>
              </NuxtLinkLocale>
            </span>
            <span class="text-sm text-gray-600 dark:text-gray-300">
              {{ t('blog.published', {date: formatPostDate(post.publishedAt, locale, 'long')}) }}
            </span>
          </div>

          <div v-if="post.description" class="my-4" :lang="articleLang">
            <p class="pb-6 font-semibold break-normal text-gray-700 text-lg md:text-xl dark:text-gray-200">
              {{ post.description }}
            </p>
          </div>

          <article class="prose md:prose-lg max-w-none dark:prose-invert text-gray-700 dark:text-gray-200 pb-8" :lang="articleLang">
            <div v-html="post.html"/>
          </article>

          <hr class="w-60 mx-auto text-gray-200 dark:text-gray-600">
        </div>

        <div v-if="post.tags && post.tags.length" class="text-gray-500 mt-6">
          <NuxtLinkLocale v-for="tag in post.tags" :key="tag" :to="`/blog/tag/${tag}`"
                    class="inline-flex items-center font-bold leading-sm px-3 py-1 mr-1 rounded-full
                           bg-gray-300 text-gray-700 dark:bg-gray-700 dark:text-gray-300">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
            </svg>
            <span class="h-6 pl-1 pt-1 mb-1 text-sm">{{ tag }}</span>
          </NuxtLinkLocale>
        </div>
      </div>
    </div>

    <div v-else class="flex flex-col items-center my-20 text-2xl text-gray-500 dark:text-gray-300">
      <span>{{ t('blog.notFound') }}</span>
      <NuxtLinkLocale to="/blog" class="mt-4 text-base text-blue-600 hover:underline dark:text-blue-400">{{ t('blog.back') }}</NuxtLinkLocale>
    </div>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const {t, locale} = useI18n()
const runtimeConfig = useRuntimeConfig()

const slug = String(route.params.slug || '')

const {data: post} = await useAsyncData(`blog-${locale.value}-${slug}`, () => fetchBlogPost(slug, locale.value))

if (!post.value) {
  throw createError({statusCode: 404, statusMessage: 'Page Not Found', fatal: true})
}

// The language of the text actually shown, for screen readers and hyphenation.
const articleLang = computed(() => (post.value?.originalOnly || locale.value === 'vi' ? 'vi' : 'en'))

const tagsStr = computed(() => (post.value?.tags || []).join(', '))
const fullUrl = computed(() => `${runtimeConfig.public.baseUrl}${route.path}`)
const fullTitle = computed(() => (post.value?.title ? `${post.value.title} | ${t('blog.siteSuffix')}` : t('blog.siteSuffix')))

useHead(() => ({
  title: fullTitle.value,
  // `keywords` is not a useSeoMeta key in Unhead v3; it belongs in raw meta.
  meta: [{name: 'keywords', content: tagsStr.value}]
}))

useSeoMeta({
  title: () => post.value?.title || '',
  description: () => post.value?.description || '',
  ogType: 'article',
  ogUrl: () => fullUrl.value,
  ogTitle: () => fullTitle.value,
  ogDescription: () => post.value?.description || '',
  ogImage: () => post.value?.cover || post.value?.thumbnail || '',
  articlePublishedTime: () => toIso(post.value?.publishedAt),
  articleModifiedTime: () => toIso(post.value?.updatedAt || post.value?.publishedAt)
})

function toIso(value: unknown): string {
  if (!value) return ''
  const d = new Date(value as string | number | Date)
  return Number.isNaN(d.getTime()) ? '' : d.toISOString()
}
</script>
```

`blog/category/[category].vue` — FIND:

```ts
const {data: posts} = await useAsyncData(`blog-cat-${locale.value}-${category.value}`, () =>
  fetchBlogPosts(locale.value).then((list) => list.filter((p) => p.category === category.value))
)
```
REPLACE:
```ts
const {data: posts} = await useAsyncData(`blog-cat-${locale.value}-${category.value}`, () =>
  fetchBlogPosts(locale.value, {category: category.value})
)
```

`blog/tag/[tag].vue` — FIND:

```ts
const {data: posts} = await useAsyncData(`blog-tag-${locale.value}-${tag.value}`, () =>
  fetchBlogPosts(locale.value).then((list) => list.filter((p) => Array.isArray(p.tags) && p.tags.includes(tag.value)))
)
```
REPLACE:
```ts
const {data: posts} = await useAsyncData(`blog-tag-${locale.value}-${tag.value}`, () =>
  fetchBlogPosts(locale.value, {tag: tag.value})
)
```

`blog/index.vue` and `blog/search.vue` need no change (`post.path`, `post.publishedAt`, `post.tags`, `post.thumbnail`, `post.description`, `post.originalOnly` all exist on `BlogPost`).

- [ ] **Step 5: Remove @nuxt/content and the translation pipeline**

FIND `apps/site/nuxt.config.ts:119-141`:

```ts
  modules: [
    '@nuxt/eslint',
    '@pinia/nuxt',
    '@vite-pwa/nuxt',
    '@nuxt/content',
    '@nuxtjs/i18n'
    // Note: robots and sitemap will be added back in next phase with proper configuration
  ],

  // @nuxt/content configuration
  content: {
    build: {
      markdown: {
        highlight: {
          theme: {
            default: 'github-light-high-contrast',
            dark: 'github-dark-high-contrast'
          },
          langs: ['js', 'ts', 'vue', 'bash', 'json', 'yaml', 'md', 'html', 'css', 'php', 'python', 'go']
        }
      }
    }
  },
```

REPLACE with:

```ts
  modules: [
    '@nuxt/eslint',
    '@pinia/nuxt',
    '@vite-pwa/nuxt',
    '@nuxtjs/i18n'
    // Note: robots and sitemap will be added back in next phase with proper configuration
  ],
```

```bash
cd apps/site
git rm -rq content.config.ts content scripts/translate-posts.mjs scripts/translate test/translateMarkdown.spec.ts app/utils/blogLocale.ts test/blogLocale.spec.ts
grep -rn "@nuxt/content\|queryCollection\|stripLocalePrefix\|mergeTranslations\|ContentRenderer" app test e2e nuxt.config.ts; echo "exit=$?"
cd ../..
```
Expected: no matches, `exit=1`.

- [ ] **Step 6: Build + e2e the blog against the local API**

```bash
cd apps/site && npm test 2>&1 | tail -3 && npm run build 2>&1 | tail -2
API_URL=http://localhost:8080 npx nuxt dev -p 3001 >/tmp/site-dev.log 2>&1 &
sleep 12
E2E_BASE_URL=http://localhost:3001 npx playwright test e2e/blog-routing.spec.ts e2e/blog-i18n.spec.ts e2e/seo.spec.ts e2e/search.spec.ts --project=desktop 2>&1 | tail -6
pkill -f 'nuxt dev -p 3001'; cd ../..
```
Expected: unit suite green except `test/stores.spec.ts` (deleted in Task 8); Playwright `passed` for all four files (`blog-routing` asserts 2 articles and the tag/category swaps, which the seed reproduces).

- [ ] **Step 7: Commit**

```bash
git add -A apps/site
git commit -m "feat(site): blog reads posts from the API; drop @nuxt/content"
```

**Done when:**
- [ ] no `@nuxt/content`, `queryCollection`, `ContentRenderer`, `stripLocalePrefix` left in `apps/site`
- [ ] `test/markdown.spec.ts` + `test/blogPosts.spec.ts` 6 passed
- [ ] `blog-routing`, `blog-i18n`, `seo`, `search` e2e pass against the local API

---

### Task 8: Social links from the API; delete the legacy blog surface

**Files:**
- Modify: `apps/site/app/components/SocialLinks.vue:3, 17-18, 61-62` (+ one `key:` line per entry)
- Delete: `apps/site/app/pages/legacy-blogs/`, `app/components/post/Detail.vue`, `app/components/post/List.vue`, `app/components/tag/ListNewest.vue`, `app/composables/useLegacyResource.ts`, `app/stores/posts.ts`, `app/stores/categories.ts`, `app/stores/tags.ts`, `app/utils/legacyState.ts`, `app/utils/legacyParams.ts`, `app/utils/legacy/`, `postcss-plugins/`, `test/legacyParams.spec.ts`, `test/legacyState.spec.ts`, `test/stores.spec.ts`, `e2e/legacy-offline.spec.ts`
- Modify: `apps/site/nuxt.config.ts` (vite.build + postcss blocks), `apps/site/package.json`, `apps/site/test/setup.ts:10-12, 30`, `e2e/{contrast,search,i18n}.spec.ts` (legacy cases only)

**Interfaces:**
- Consumes: `useSiteSettings()` (Task 6).
- Produces: `SocialLinks.vue` renders hrefs from `site_settings.social_links` keyed `mail|telegram|signal|discord|github|facebook|linkedin`, bundled hrefs as fallback.

- [ ] **Step 1: SocialLinks from settings**

FIND `SocialLinks.vue:3` `    <li v-for="item in LINKS" :key="item.label">` → REPLACE `    <li v-for="item in links" :key="item.key">`.

FIND `SocialLinks.vue:17-18`:

```ts
// Icon paths are the ones that lived inline in Footer.vue and about/index.vue.
const LINKS = [
```
REPLACE:
```ts
// Icon paths are the ones that lived inline in Footer.vue and about/index.vue.
// hrefs here are the fallback; qdjr-admin → Settings → Social links overrides
// them by key (and hides any key it does not list).
const DEFAULT_LINKS = [
```

Then give each entry a `key`. Seven exact FIND/REPLACE pairs (each label line is unique in the file):

| FIND | REPLACE |
|---|---|
| `    label: 'Mail',` | `    key: 'mail',` + newline + `    label: 'Mail',` |
| `    label: 'Telegram',` | `    key: 'telegram',` + newline + `    label: 'Telegram',` |
| `    label: 'Signal',` | `    key: 'signal',` + newline + `    label: 'Signal',` |
| `    label: 'Discord',` | `    key: 'discord',` + newline + `    label: 'Discord',` |
| `    label: 'Github',` | `    key: 'github',` + newline + `    label: 'Github',` |
| `    label: 'Facebook',` | `    key: 'facebook',` + newline + `    label: 'Facebook',` |
| `    label: 'Linkedin',` | `    key: 'linkedin',` + newline + `    label: 'Linkedin',` |

Equivalent one-liner (run from repo root):

```bash
perl -0pi -e "s/(\n)(    label: '(Mail|Telegram|Signal|Discord|Github|Facebook|Linkedin)',)/\$1    key: '\L\$3\E',\$1\$2/g" apps/site/app/components/SocialLinks.vue
grep -c "    key: '" apps/site/app/components/SocialLinks.vue
```
Expected: `7`.

FIND `SocialLinks.vue:61-62`:

```ts
] as const
</script>
```
REPLACE:
```ts
] as const

const settings = useSiteSettings()
const links = computed(() => {
  const remote = settings.value?.social_links ?? {}
  const hasRemote = Object.keys(remote).length > 0
  return DEFAULT_LINKS.filter((item) => !hasRemote || remote[item.key]).map((item) => ({...item, href: remote[item.key] ?? item.href}))
})
</script>
```

- [ ] **Step 2: Delete the legacy surface**

```bash
cd apps/site
git rm -rq app/pages/legacy-blogs app/components/post/Detail.vue app/components/post/List.vue app/components/tag/ListNewest.vue app/composables/useLegacyResource.ts app/stores/posts.ts app/stores/categories.ts app/stores/tags.ts app/utils/legacyState.ts app/utils/legacyParams.ts app/utils/legacy postcss-plugins test/legacyParams.spec.ts test/legacyState.spec.ts test/stores.spec.ts e2e/legacy-offline.spec.ts
npm uninstall prismjs video.js vue-spinner
grep -n "legacy" e2e/contrast.spec.ts e2e/search.spec.ts e2e/i18n.spec.ts
cd ../..
```
The only hit is `e2e/contrast.spec.ts:8`. FIND:

```ts
const PAGES = ['/', '/about', '/blog', '/blog/hello-world', '/blog/search?q=zzzz', '/legacy-blogs', '/trang-khong-ton-tai']
```
REPLACE:
```ts
const PAGES = ['/', '/about', '/blog', '/blog/hello-world', '/blog/search?q=zzzz', '/trang-khong-ton-tai']
```
Expected afterwards: `grep -rn legacy apps/site/e2e` prints nothing.

- [ ] **Step 3: nuxt.config and test setup**

Replace `apps/site/nuxt.config.ts` with its full final content (Tasks 3, 6, 7 edits plus this task's removal of the legacy-only `vite.build` threshold and the prismjs PostCSS rewrite):

```ts
import tailwindcss from '@tailwindcss/vite'

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  // Enable TypeScript (less strict for development)
  typescript: {
    strict: false,
    typeCheck: false,
    // Nuxt 4 enables noUncheckedIndexedAccess unconditionally — it is tied to the
    // v4 compatibility version, NOT to `strict`. Pinning it false keeps the
    // typecheck surface comparable to Nuxt 3 so any new error is genuinely a Nuxt
    // 4 issue. Follow-up: flip this and `strict` together, deliberately.
    tsConfig: {
      compilerOptions: {
        noUncheckedIndexedAccess: false
      }
    }
  },

  // Add compatibility date
  compatibilityDate: '2026-08-16',

  // Development tools
  devtools: { enabled: true },

  // Runtime config (replaces env and publicRuntimeConfig)
  runtimeConfig: {
    // Private keys (only available on server-side)
    // Public keys (exposed to client-side)
    public: {
      baseUrl: process.env.APP_URL || 'https://qdjr.dev',
      // qdjr-admin API (apps/api). Netlify sets API_URL; local dev uses the Go
      // server on :8080.
      apiUrl: process.env.API_URL || 'https://api.qdjr.dev',
      gaId: process.env.GOOGLE_ANALYTICS_ID || 'G-KBZQ6KNY8T'
    }
  },

  // App config (replaces head configuration)
  app: {
    head: {
      // The default vibe (stores/theme.ts) in the page shell, so its tokens apply before any script runs.
      htmlAttrs: {'data-vibe': 'cartoon'},
      title: 'QDJr Blog',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        {
          name: 'description',
          content: 'This is a small blog of Nguyen Quoc Dai'
        },
        {
          name: 'keywords',
          content: 'development, software, qdjr, quocdaijr, nguyen quoc dai, blog'
        },
        {
          property: 'og:url',
          content: process.env.APP_URL || 'https://qdjr.dev'
        },
        {
          property: 'og:type',
          content: 'website'
        },
        {
          property: 'og:site_name',
          content: 'QDJr'
        },
        {
          property: 'og:title',
          content: 'QDJr Blog'
        },
        {
          property: 'og:description',
          content: 'This is a small blog of Nguyen Quoc Dai'
        },
        {
          property: 'og:image',
          content: (process.env.APP_URL || 'https://qdjr.dev') + '/profile.jpg'
        },
        { name: 'apple-mobile-web-app-title', content: 'QDJr Blog' },
        { name: 'format-detection', content: 'telephone=no' }
      ],
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
        // Vibe fonts. Three static stylesheets (one per vibe) rather than a
        // runtime swap: browsers only download font binaries for text that is
        // actually rendered, so the inactive vibes cost one small CSS fetch each.
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap'
        },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght,SOFT,WONK@9..144,700,100,1&family=Bricolage+Grotesque:opsz,wght@12..96,400;12..96,600&display=swap'
        },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Chakra+Petch:wght@600&family=Geist:wght@400;600&family=Geist+Mono:wght@400&display=swap'
        }
      ]
    }
  },

  // Global CSS — Tailwind v4 entrypoint
  css: ['~/assets/css/main.css'],

  // Auto import components
  components: true,

  // Modules (buildModules merged into modules in Nuxt 3)
  modules: [
    '@nuxt/eslint',
    '@pinia/nuxt',
    '@vite-pwa/nuxt',
    '@nuxtjs/i18n'
    // Note: robots and sitemap will be added back in next phase with proper configuration
  ],

  // Vietnamese at /, English under /en. Browsers whose preferred language is
  // English are redirected once from / to /en; the cookie remembers a manual
  // choice made with the header language switch.
  i18n: {
    strategy: 'prefix_except_default',
    defaultLocale: 'vi',
    baseUrl: process.env.APP_URL || 'https://qdjr.dev',
    locales: [
      {code: 'vi', language: 'vi-VN', name: 'Tiếng Việt', file: 'vi.json'},
      {code: 'en', language: 'en-US', name: 'English', file: 'en.json'}
    ],
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'i18n_lang',
      redirectOn: 'root',
      alwaysRedirect: false,
      fallbackLocale: 'vi'
    }
  },

  // PWA configuration (updated for @vite-pwa/nuxt)
  pwa: {
    manifest: {
      lang: 'en'
    },
    workbox: {
      // Server-rendered pages: no app shell to fall back to, so no
      // navigateFallback (serving '/' for every URL would hydrate the wrong page).
      navigateFallback: null,
      // No precache: a low-traffic portfolio, and the old SPA-mode glob warnings.
      globPatterns: []
    }
  },

  // Server-rendered on request (the trip planner needs the Nitro server anyway),
  // so the first paint is real HTML instead of waiting for the app bundle.
  ssr: true,

  hooks: {
    // No <link rel="prefetch"> for lazy chunks in the server-rendered HTML: on
    // a slow phone connection three.js and the scene (650 KB) fought the page's
    // own CSS and fonts for bandwidth. They load when asked for (VibeScene
    // mounts after load), and NuxtLink still prefetches pages on sight.
    'build:manifest': (manifest) => {
      for (const chunk of Object.values(manifest)) chunk.prefetch = false
    }
  },

  // Pages render live, nothing is prerendered: no payload files or app manifest needed.
  experimental: {
    payloadExtraction: false,
    appManifest: false
  },

  // Nitro configuration
  nitro: {
    // Planned trips (/api/trips/plan), kept 30 days on disk across restarts.
    storage: {
      trips: {driver: 'fs', base: './.data/trips'}
    },
    prerender: {
      routes: []
    }
  },

  // Tailwind v4. Replaces @nuxtjs/tailwindcss, which is pinned to tailwindcss
  // ~3.4 and has no v4 support.
  vite: {
    plugins: [tailwindcss()]
  }
})
```

Trap: Netlify's Nitro preset stores `useStorage('trips')` on an ephemeral function filesystem, so planned trips are cached per function instance only. That is acceptable (the cache is an optimisation) and is listed under *Deliberately not done*.

FIND `apps/site/test/setup.ts:10-12`:

```ts
// The object returned by useNuxtApp(). Tests overwrite `$api` per case via
// setMockApi() below.
const nuxtApp: Record<string, unknown> = {$api: {enabled: false}}
```
REPLACE:
```ts
// The object returned by useNuxtApp(). Tests overwrite `$api` per case via
// setMockApi() below.
const nuxtApp: Record<string, unknown> = {$api: {}}
```
and FIND `setup.ts:30` `  setMockApi({enabled: false})` → REPLACE `  setMockApi({})`.

- [ ] **Step 4: Verify nothing references the deleted code**

```bash
cd apps/site
grep -rn "legacy\|prismjs\|video\.js\|vue-spinner\|usePostsStore\|useCategoriesStore\|useTagsStore\|PostList\|PostDetail\|TagListNewest\|rewrite-color-adjust" app nuxt.config.ts test vitest.config.ts package.json | grep -v "app/components/Header.vue"; echo "exit=$?"
npm run lint:js 2>&1 | tail -3 && npm test 2>&1 | tail -3 && npm run build 2>&1 | tail -2
cd ../..
```
Expected: `exit=1` (only `Header.vue`'s old TODO comment may mention a store; leave it), lint clean, all unit tests pass, build OK.

- [ ] **Step 5: Commit**

```bash
git add -A apps/site
git commit -m "refactor(site): social links from site settings; remove legacy blog surface"
```

**Done when:**
- [ ] `/legacy-blogs` returns 404; no legacy component, store, util or test remains
- [ ] footer and /about social links follow `site_settings.social_links` (edit in admin → reload)
- [ ] `npm run lint:js`, `npm test`, `npm run build` green; `grep -rn legacy apps/site/e2e` empty

---

### Task 9: API on Render; CI = test only; keepalive

**Files:**
- Create: `render.yaml`, `.github/workflows/keepalive.yml`
- Modify: `apps/api/Dockerfile:3`, `.github/workflows/api.yml`
- Delete: `infra/cloud-run/`

- [ ] **Step 1: Go 1.25 everywhere**

FIND `apps/api/Dockerfile:3` `FROM golang:1.23-alpine AS build` → REPLACE `FROM golang:1.25-alpine AS build`.

Replace `.github/workflows/api.yml` entirely:

```yaml
name: api

on:
  push:
    branches: [main]
    paths:
      - 'apps/api/**'
      - '.github/workflows/api.yml'
  pull_request:
    branches: [main]
    paths:
      - 'apps/api/**'
      - '.github/workflows/api.yml'

permissions:
  contents: read

jobs:
  test:
    name: test + lint
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/api
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          cache: true
          cache-dependency-path: apps/api/go.sum

      - name: Test
        run: go test -race ./...

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          working-directory: apps/api

# Deploys are done by Render from render.yaml (autoDeployTrigger: commit); the
# API applies its embedded goose migrations at startup (RUN_MIGRATIONS=true).
```

- [ ] **Step 2: Render Blueprint**

`render.yaml` (repo root):

```yaml
# Render Blueprint for the Go API. Connect this repo once in the Render
# dashboard (New → Blueprint); every push to main then redeploys.
# Secrets marked sync: false are entered in the dashboard on first sync.
services:
  - type: web
    name: qdjr-admin-api
    runtime: docker
    plan: free
    region: singapore
    rootDir: apps/api
    dockerfilePath: ./apps/api/Dockerfile
    dockerContext: ./apps/api
    healthCheckPath: /healthz
    autoDeployTrigger: commit
    envVars:
      - key: ENV
        value: production
      - key: PORT
        value: "8080"
      # Embedded goose migrations run before the server listens; a failing
      # migration fails the health check and Render keeps the previous deploy.
      - key: RUN_MIGRATIONS
        value: "true"
      - key: SUPABASE_URL
        value: https://rknqbtaybeqdzwwonlmg.supabase.co
      - key: SUPABASE_JWKS_URL
        value: https://rknqbtaybeqdzwwonlmg.supabase.co/auth/v1/.well-known/jwks.json
      # Admin origin: append the workers.dev URL printed by the first Cloudflare
      # deploy (Task 10) and push; Render re-syncs the blueprint.
      - key: CORS_ORIGINS
        value: https://qdjr.dev,https://www.qdjr.dev
      # Supabase *Session* pooler URL (port 5432, IPv4): the direct host is
      # IPv6-only and transaction pooling breaks pgx prepared statements.
      - key: DATABASE_URL
        sync: false
      - key: SUPABASE_JWT_SECRET
        sync: false
      - key: SUPABASE_SERVICE_ROLE_KEY
        sync: false
```

Trap: Render documents `dockerfilePath`/`dockerContext` as repo-root relative. If the first build reports `Dockerfile not found`, change them to `./Dockerfile` and `.` (rootDir-relative) and push.

- [ ] **Step 3: Keepalive**

`.github/workflows/keepalive.yml`:

```yaml
name: keepalive

# Render free web services sleep after 15 idle minutes (≈30 s cold start on
# every SSR page of qdjr.dev). A ping every 10 min keeps the API warm; one
# always-on free service fits the 750 instance-hours/month allowance.
# GitHub disables scheduled workflows after 60 days without repo activity —
# re-enable from the Actions tab if the API starts cold-starting again.
on:
  schedule:
    - cron: '*/10 * * * *'
  workflow_dispatch:

jobs:
  ping:
    runs-on: ubuntu-latest
    steps:
      - run: curl --fail --silent --max-time 60 https://api.qdjr.dev/healthz
```

- [ ] **Step 4: Local Docker build proves the Dockerfile and the startup migration**

```bash
docker build -q -t qdjr-admin-api:local apps/api
docker run --rm --network host -e PORT=8090 -e RUN_MIGRATIONS=true -e DATABASE_URL='postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable' -e SUPABASE_URL=http://127.0.0.1:54321 -e SUPABASE_JWT_SECRET=super-secret-jwt-token-with-at-least-32-characters-long qdjr-admin-api:local 2>&1 | head -4 &
sleep 5; curl -s localhost:8090/healthz; pkill -f 'qdjr-admin-api:local' || docker ps -q --filter ancestor=qdjr-admin-api:local | xargs -r docker stop
```
Expected: image builds on Go 1.25; logs show `config loaded`, `goose: no migrations to run. current version: 9`, `migrations applied`, `listening`; healthz returns the ok envelope. Trap: on macOS `--network host` does not reach the host; use `-e DATABASE_URL='postgres://postgres:postgres@host.docker.internal:54322/…'` and `-p 8090:8090` instead.

- [ ] **Step 5: Commit**

```bash
git rm -rq infra/cloud-run
git add render.yaml .github/workflows/api.yml .github/workflows/keepalive.yml apps/api/Dockerfile
git commit -m "ci(api): deploy via Render blueprint; Go 1.25; keepalive ping"
```

**Done when:**
- [ ] `docker build apps/api` succeeds on golang:1.25
- [ ] container with `RUN_MIGRATIONS=true` logs `migrations applied` then `listening`, and `/healthz` returns ok
- [ ] `infra/cloud-run/` deleted; `api.yml` has no deploy job; `render.yaml` and `keepalive.yml` committed

---

### Task 10: Admin on Cloudflare Workers (OpenNext)

**Files:**
- Modify: `apps/web/package.json`, `apps/web/next.config.ts`, `apps/web/.gitignore`, `apps/web/app/page.tsx`
- Create: `apps/web/wrangler.jsonc`, `apps/web/open-next.config.ts`, `apps/web/public/_headers`
- Delete: `apps/web/vercel.json`
- Modify: `.github/workflows/web.yml`

**Interfaces:**
- Produces: `pnpm preview` serves the admin on `http://localhost:8787` from the Workers runtime; `pnpm deploy` publishes `qdjr-admin.<account>.workers.dev`. CI deploy needs repo secrets `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` and repo variables `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`.

- [ ] **Step 1: Bump Next and add the adapter**

```bash
cd apps/web
pnpm add next@16.4.0 eslint-config-next@16.4.0 @opennextjs/cloudflare@^1.20.10
pnpm add -D wrangler@^4.149.0
grep -nE '"(next|eslint-config-next|@opennextjs/cloudflare|wrangler)"' package.json
cd ../..
```
Expected: `next` and `eslint-config-next` at `16.4.0`, adapter `^1.20.10`, wrangler `^4.149.0`.

- [ ] **Step 2: Config files**

`apps/web/wrangler.jsonc`:

```jsonc
{
  "$schema": "node_modules/wrangler/config-schema.json",
  "name": "qdjr-admin",
  "main": ".open-next/worker.js",
  "compatibility_date": "2025-09-01",
  "compatibility_flags": ["nodejs_compat", "global_fetch_strictly_public"],
  "workers_dev": true,
  "assets": {
    "directory": ".open-next/assets",
    "binding": "ASSETS"
  },
  "services": [
    { "binding": "WORKER_SELF_REFERENCE", "service": "qdjr-admin" }
  ]
  // Custom domain later: "routes": [{ "pattern": "admin.qdjr.dev", "custom_domain": true }]
  // — requires the qdjr.dev zone on Cloudflare (DNS is on Netlify today).
}
```

`apps/web/open-next.config.ts`:

```ts
import { defineCloudflareConfig } from '@opennextjs/cloudflare';

// No incremental cache: every admin page is client-rendered behind auth.
export default defineCloudflareConfig();
```

`apps/web/public/_headers`:

```
/_next/static/*
  Cache-Control: public,max-age=31536000,immutable
```

Replace `apps/web/next.config.ts`:

```ts
import type { NextConfig } from 'next';
import { initOpenNextCloudflareForDev } from '@opennextjs/cloudflare';

// Lets `next dev` see Cloudflare bindings; a no-op in production builds.
initOpenNextCloudflareForDev();

const nextConfig: NextConfig = {};

export default nextConfig;
```

Replace `apps/web/app/page.tsx` (the Next starter page):

```tsx
import { redirect } from 'next/navigation';

export default function Home() {
  redirect('/admin');
}
```

Append to `apps/web/.gitignore`:

```
# opennext / wrangler
.open-next
.wrangler
```

In `apps/web/package.json` FIND:

```json
    "start": "next start",
```
REPLACE:
```json
    "start": "next start",
    "preview": "opennextjs-cloudflare build && opennextjs-cloudflare preview",
    "deploy": "opennextjs-cloudflare build && opennextjs-cloudflare deploy",
    "cf-typegen": "wrangler types --env-interface CloudflareEnv cloudflare-env.d.ts",
```

```bash
git rm -q apps/web/vercel.json
```

- [ ] **Step 3: Workflow with a deploy job**

Replace `.github/workflows/web.yml`:

```yaml
name: web

on:
  push:
    branches: [main]
    paths:
      - 'apps/web/**'
      - '.github/workflows/web.yml'
  pull_request:
    branches: [main]
    paths:
      - 'apps/web/**'
      - '.github/workflows/web.yml'

permissions:
  contents: read

env:
  # Public build-time values (inlined into the bundle). Set as repository
  # variables: Settings → Secrets and variables → Actions → Variables.
  NEXT_PUBLIC_API_URL: ${{ vars.NEXT_PUBLIC_API_URL }}
  NEXT_PUBLIC_SUPABASE_URL: ${{ vars.NEXT_PUBLIC_SUPABASE_URL }}
  NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY: ${{ vars.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY }}

jobs:
  checks:
    name: typecheck + lint + build
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/web
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 10
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'pnpm'
          cache-dependency-path: apps/web/pnpm-lock.yaml
      - run: pnpm install --frozen-lockfile
      - run: pnpm exec tsc --noEmit
      - run: pnpm lint
      - run: pnpm build

  deploy:
    name: deploy to Cloudflare Workers
    needs: checks
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/web
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 10
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'pnpm'
          cache-dependency-path: apps/web/pnpm-lock.yaml
      - run: pnpm install --frozen-lockfile
      - run: pnpm exec opennextjs-cloudflare build
      - run: pnpm exec wrangler deploy
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
```

- [ ] **Step 4: Verify locally in the Workers runtime**

```bash
cd apps/web && pnpm exec tsc --noEmit && pnpm lint && pnpm build 2>&1 | tail -3
pnpm exec opennextjs-cloudflare build 2>&1 | tail -3
(pnpm exec opennextjs-cloudflare preview >/tmp/cf-preview.log 2>&1 &) ; sleep 10
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' http://localhost:8787/
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8787/login
pkill -f 'wrangler dev'; cd ../..
```
Expected: `307 http://localhost:8787/admin` then `200`. Trap: `.env.local` is read by `next build`, so the preview talks to `http://localhost:8080`; fine for the smoke.

- [ ] **Step 5: Commit**

```bash
git add apps/web/package.json apps/web/pnpm-lock.yaml apps/web/next.config.ts apps/web/wrangler.jsonc apps/web/open-next.config.ts apps/web/public/_headers apps/web/app/page.tsx apps/web/.gitignore .github/workflows/web.yml
git commit -m "ci(web): deploy the admin to Cloudflare Workers with OpenNext; Next 16.4"
```

**Done when:**
- [ ] `next` and `eslint-config-next` are 16.4.0; `vercel.json` deleted
- [ ] `opennextjs-cloudflare preview` serves `/` → 307 `/admin` and `/login` → 200
- [ ] `web.yml` has `checks` + `deploy` jobs; `tsc`, `lint`, `build` green

---

### Task 11: Netlify config and documentation

**Files:**
- Create: `netlify.toml`, `docs/superpowers/setup/2026-10-12-deployment-checklist.md`
- Replace: `infra/dns/README.md`, `.claude/commands/deploy-api.md`
- Modify: `CLAUDE.md:7-8, 17, 25-27, 43-48`, `README.md` (overview diagram, stack table, layout, Local dev, Deploy section, Plan 3 row)
- Delete: `docs/superpowers/setup/2026-04-27-deployment-checklist.md`

- [ ] **Step 1: Netlify**

`netlify.toml` (repo root):

```toml
# Netlify builds the public site from this monorepo. In the Netlify UI point the
# existing qdjr.dev site at quocdaijr/qdjr-admin (branch main); base/command/
# publish below take effect from there. Env: APP_URL=https://qdjr.dev,
# API_URL=https://api.qdjr.dev.
[build]
  base = "apps/site"
  command = "npm run build"
  publish = "dist"
```

- [ ] **Step 2: DNS and deploy command**

Replace `infra/dns/README.md`:

```markdown
# DNS for qdjr.dev

Zone: `qdjr.dev`, registrar Hostinger, DNS hosted by Netlify (NS1 nameservers).
Manage records at app.netlify.com → Domains → qdjr.dev.

| Host | Type | Target | Serves |
|---|---|---|---|
| `qdjr.dev`, `www` | (existing) | Netlify site | public site (`apps/site`) |
| `api` | CNAME | `qdjr-admin-api.onrender.com` | Go API (Render). Add the custom domain in Render → qdjr-admin-api → Settings → Custom Domains first; Render issues the certificate once the CNAME resolves. |

The admin UI has no record for now: it is served at `qdjr-admin.<account>.workers.dev`.
To move it to `admin.qdjr.dev` later, the whole zone has to be on Cloudflare
(Workers custom domains require that): change nameservers at Hostinger, recreate
the Netlify records in Cloudflare, then add `"routes": [{"pattern": "admin.qdjr.dev", "custom_domain": true}]`
to `apps/web/wrangler.jsonc` and the origin to `CORS_ORIGINS` in `render.yaml`.
```

Replace `.claude/commands/deploy-api.md`:

````markdown
---
description: Deploy the API (Render redeploys from main; this is the manual path)
---

Render deploys `qdjr-admin-api` on every push to `main` (`render.yaml`, Docker build of `apps/api`), applying embedded goose migrations at startup. To redeploy by hand: Render dashboard → qdjr-admin-api → Manual Deploy → "Deploy latest commit". Check:

```bash
curl -s https://api.qdjr.dev/healthz && curl -s https://api.qdjr.dev/readyz
```
````

- [ ] **Step 3: Checklist**

`docs/superpowers/setup/2026-10-12-deployment-checklist.md`:

```markdown
# Deployment checklist (one-time, manual)

Pairs with `render.yaml`, `.github/workflows/web.yml`, `netlify.toml`. Work top to bottom.

## Supabase (`rknqbtaybeqdzwwonlmg`)

- [ ] Dashboard → Connect → copy the **Session pooler** URI (port 5432, `?sslmode=require`); this is `DATABASE_URL` for Render. Not the direct host (IPv6-only) and not the transaction pooler (6543).
- [ ] Note the JWT secret, service-role key and publishable (anon) key from Settings → API.
- [ ] Baseline check, once: SQL editor → `select count(*) from information_schema.tables where table_schema = 'public';`
      - `0` → nothing to do; the API's first start applies 0001–0009.
      - `> 0` (an earlier `supabase db push` ran) → tell goose those are applied before the first deploy:
        ```sql
        create table if not exists public.goose_db_version (
          id serial primary key, version_id bigint not null, is_applied boolean not null, tstamp timestamp default now());
        insert into public.goose_db_version (version_id, is_applied)
        values (0,true),(1,true),(2,true),(3,true),(4,true),(5,true),(6,true);
        ```
        then run the old `supabase/seed.sql` inserts by hand or let 0007 handle them (it is `on conflict do nothing`).

## Render (API)

- [ ] Dashboard → New → Blueprint → connect `quocdaijr/qdjr-admin`, branch `main`; it reads `render.yaml`
- [ ] Enter `DATABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_SERVICE_ROLE_KEY` when prompted
- [ ] First deploy green; Logs show `migrations applied` → `curl https://qdjr-admin-api.onrender.com/healthz` → `{"data":{"status":"ok"},…}`
- [ ] Settings → Custom Domains → add `api.qdjr.dev`; then add the CNAME at Netlify DNS (see `infra/dns/README.md`)
- [ ] Bootstrap the first super_admin against prod (from `apps/api`, with the prod env exported):
      `BOOTSTRAP_ADMIN_EMAIL=… BOOTSTRAP_ADMIN_PASSWORD=… go run ./cmd/bootstrap`

## Cloudflare (admin)

- [ ] Dashboard → My Profile → API Tokens → Create → template "Edit Cloudflare Workers"; copy the token and the Account ID (Workers & Pages overview)
- [ ] GitHub secrets: `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`
- [ ] GitHub variables: `NEXT_PUBLIC_API_URL=https://api.qdjr.dev`, `NEXT_PUBLIC_SUPABASE_URL=https://rknqbtaybeqdzwwonlmg.supabase.co`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=<publishable key>`
- [ ] Push to `main` → `web.yml` deploy job prints the URL `https://qdjr-admin.<account>.workers.dev`
- [ ] Append that origin to `CORS_ORIGINS` in `render.yaml`, push (Render re-syncs)
- [ ] Supabase → Authentication → URL Configuration → add the workers.dev URL to Redirect URLs

## Netlify (site)

- [ ] Site settings → Build & deploy → Repository → switch to `quocdaijr/qdjr-admin`, branch `main` (base dir `apps/site` comes from `netlify.toml`)
- [ ] Environment variables: `APP_URL=https://qdjr.dev`, `API_URL=https://api.qdjr.dev`
- [ ] Trigger deploy → `https://qdjr.dev/blog` lists the two seeded posts from the API
- [ ] Archive the old `quocdaijr/qdjr` repo (GitHub → Settings → Archive) so nobody pushes there

## Done when

- [ ] `curl https://api.qdjr.dev/v1/posts | jq '.meta.total'` → `2`
- [ ] admin login at the workers.dev URL shows the `super_admin` badge; `/admin/profile` shows the seeded JSON
- [ ] `https://qdjr.dev/about` renders; editing the JSON in the admin and reloading `/about` shows the change
```

```bash
git rm -q docs/superpowers/setup/2026-04-27-deployment-checklist.md
```

- [ ] **Step 4: CLAUDE.md**

Replace `CLAUDE.md` entirely:

````markdown
# qdjr-admin

Admin dashboard backend (Go/Gin), admin UI (Next.js) and public site (Nuxt) for qdjr.dev, backed by Supabase. Replaces legacy `cmsqdjr` (Laravel). Single-source-of-truth design lives in `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`.

## Tech (pinned)

- **API:** Go 1.25, Gin, `pgx/v5`, goose migrations embedded and applied at startup, deployed to Render (Docker, `render.yaml`, region singapore) at `api.qdjr.dev`
- **Web:** Next.js 16 (App Router) + shadcn/ui + TanStack Query/Table + react-hook-form + zod, deployed to Cloudflare Workers via `@opennextjs/cloudflare` (`qdjr-admin.<account>.workers.dev`). Look is locked in `design.md` (Hallmark · Cobalt).
- **Site:** `apps/site`, Nuxt 4 public site for `qdjr.dev`, deployed by Netlify (`netlify.toml`); reads posts/taxonomy/profile/settings from the API
- **Data:** Supabase project `rknqbtaybeqdzwwonlmg` (Postgres + Auth + Storage)
- **Tooling:** goose for migrations, Supabase CLI only for the local stack, GitHub Actions for CI

## Layout

```
apps/api/              # Go backend
apps/api/migrations/   # goose SQL — single source of truth for schema (embedded in the API)
apps/web/              # Next.js admin UI
apps/site/             # Nuxt 4 public site (qdjr.dev)
design.md              # locked admin design system — read before any UI change
docs/superpowers/      # specs + plans
```

## Conventions

- **Response envelope (always):** `{ "data": ..., "meta": { ... }, "error": null }` for success; `{ "data": null, "error": { "code": "...", "message": "..." } }` for failure. HTTP status set correctly.
- **Error codes:** SCREAMING_SNAKE_CASE (`UNAUTHENTICATED`, `FORBIDDEN`, `NOT_FOUND`, `VALIDATION`, `RATE_LIMITED`, `INTERNAL`).
- **Middleware order:** `RequireAuth → RequirePermission(...) → RequireOwnership(...)` (last only for the `author` role on resource-scoped routes).
- **Migrations are forward-only** (goose, `apps/api/migrations/00NN_<name>.sql`, `-- +goose Up` only). Rollback = a new migration. Never edit a committed migration. Locally: `/db-reset`; production: applied by the API at startup (`RUN_MIGRATIONS=true`).
- **Slugs:** lowercase, hyphenated, ASCII only, max 200 chars.
- **Immutable updates in handlers:** never mutate input structs; build a new value to write.
- **Admin UI:** colours and fonts only through the tokens in `apps/web/app/globals.css`; no raw colour classes (`zinc-*`, `red-600`, hex). New screens follow `design.md`.

## Local dev

```bash
supabase start                                                                 # local Postgres + Auth + Storage on Docker
cd apps/api && cp .env.example .env && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up   # goose migrations
cd apps/api && go run ./cmd/api                                                # API on :8080
cd apps/web && cp .env.example .env.local && pnpm dev                          # admin on :3000
cd apps/site && cp .env.example .env && API_URL=http://localhost:8080 npm run dev -- -p 3001   # site on :3001
```

## See also

- Design spec: `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`
- Admin design system: `design.md`
- Active plan: `docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md`

## Deploy

- `git push origin main` triggers: `api.yml` (test + lint; Render redeploys on its own from `render.yaml` and applies migrations at startup), `web.yml` (checks, then `wrangler deploy` to Cloudflare), `keepalive.yml` (10-min healthz ping so the free Render instance never sleeps). Netlify builds `apps/site` from its own Git integration.
- Logs: Render dashboard → qdjr-admin-api → Logs; Cloudflare → Workers & Pages → qdjr-admin → Logs; Netlify → Deploys.
- Manual admin deploy: `cd apps/web && pnpm deploy` (needs `wrangler login`).
- See `infra/dns/README.md` for DNS and `docs/superpowers/setup/2026-10-12-deployment-checklist.md` for one-time platform setup.
````

- [ ] **Step 5: README**

Replace `README.md` entirely:

````markdown
# qdjr-admin

Admin API (Go/Gin), admin UI (Next.js) and public site (Nuxt) for [qdjr.dev](https://qdjr.dev), backed by Supabase. Replaces the legacy `cmsqdjr` Laravel CMS.

[![Plan 1](https://img.shields.io/badge/Plan%201-Foundation-success)](docs/superpowers/plans/2026-04-27-qdjr-admin-foundation.md)
[![Plan 2](https://img.shields.io/badge/Plan%202-Resources%20%2B%20UI-success)](docs/superpowers/plans/2026-04-27-qdjr-admin-resources.md)
[![Plan 3](https://img.shields.io/badge/Plan%203-Production-success)](docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md)

---

## Overview

A small, single-source-of-truth publishing stack that costs $0 on free tiers.

```
┌──────────────┐        ┌────────────────────┐        ┌────────────────┐
│  apps/site   │◄──────►│  apps/api          │◄──────►│   Supabase     │
│  Nuxt 4 SSR  │  REST  │  Go 1.25 + Gin     │  pgx   │   Postgres     │
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
| **API** | Go 1.25 · Gin · `pgx/v5` · `golang-jwt/v5` · `pressly/goose` (embedded migrations) · Supabase JWKS verification · Render (Docker, singapore) |
| **Web** | Next.js 16 (App Router · Turbopack) · React 19 · TypeScript · Tailwind 4 · shadcn/ui (base-ui) · TanStack Query/Table · react-hook-form + zod · Cloudflare Workers (`@opennextjs/cloudflare`) · design system in `design.md` |
| **Site** | Nuxt 4 · Vue 3 · Tailwind 4 · three.js · markdown-it · Netlify |
| **Data** | Supabase Postgres 17 · Supabase Auth (email + JWKS) · Supabase Storage |
| **CI/CD** | GitHub Actions (api / web / keepalive) · Render Blueprint · goose migrations applied by the API at startup |

## Repo layout

```
qdjr-admin/
├── apps/
│   ├── api/                 # Go backend (Gin + pgx)
│   │   ├── cmd/api/         # main entrypoint (runs migrations when RUN_MIGRATIONS=true)
│   │   ├── cmd/migrate/     # goose CLI: up · status · version · up-to N
│   │   ├── cmd/bootstrap/   # one-shot CLI to seed first super_admin
│   │   ├── migrations/      # goose SQL migrations (embedded)
│   │   └── internal/        # config, db, migrate, auth, rbac, http, posts, categories, tags, media, profile, sitesettings, contact, users, adminapi
│   ├── web/                 # Next.js 16 admin UI
│   │   ├── app/             # App Router pages (login, admin/*)
│   │   ├── components/      # page-header, status-chip, shadcn primitives in ui/
│   │   ├── lib/             # API client, auth client, types
│   │   └── e2e/             # Playwright smoke tests
│   └── site/                # Nuxt 4 public site (qdjr.dev)
├── supabase/
│   └── config.toml          # local Postgres only (supabase start)
├── infra/
│   └── dns/                 # DNS record reference
├── docs/superpowers/
│   ├── specs/               # design docs
│   ├── plans/               # implementation plans
│   └── setup/               # one-time platform setup checklist
├── design.md                # locked admin design system (Cobalt)
├── render.yaml              # Render Blueprint (API)
├── netlify.toml             # Netlify build config (site)
└── .github/workflows/       # api.yml · web.yml · keepalive.yml
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

Prerequisites: Go 1.25+, Node 22+, pnpm 10+, Docker, [Supabase CLI](https://supabase.com/docs/guides/local-development/cli/getting-started).

```bash
# 1. Start the local Supabase stack (Postgres + Auth + Storage)
supabase start

# 2. Apply migrations (goose, embedded in the API)
cd apps/api
cp .env.example .env
# Fill BOOTSTRAP_ADMIN_EMAIL, BOOTSTRAP_ADMIN_PASSWORD, and the values from `supabase status --output env`
env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up

# 3. Bootstrap a super_admin (creates auth user + assigns role)
env $(grep -v '^#' .env | xargs) go run ./cmd/bootstrap

# 4. Run the API on :8080
go run ./cmd/api

# 5. In another terminal, run the admin UI on :3000
cd apps/web
cp .env.example .env.local
pnpm install
pnpm dev

# 6. In a third terminal, run the public site on :3001
cd apps/site
cp .env.example .env
npm install
API_URL=http://localhost:8080 npm run dev -- -p 3001
```

Open http://localhost:3000/login and sign in as the bootstrap user. Reset everything with `supabase db reset` then step 2 again.

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

# Site — unit + E2E (requires API + site dev server running)
cd apps/site
npm test && E2E_BASE_URL=http://localhost:3001 npx playwright test --project=desktop
```

## Status

| Plan | Scope | Status |
|---|---|---|
| **Plan 1 — Foundation** | Schema + Go skeleton + auth + RBAC + `/v1/admin/me` + bootstrap CLI | ✅ shipped @ `v0.1.0-foundation` |
| **Plan 2 — Resources + UI** | All 8 BE resources + admin Web UI + Playwright e2e | ✅ shipped @ `v0.2.0-resources` |
| **Plan 3 — Production** | goose + Render + Cloudflare + Netlify + site cutover + admin redesign | ✅ code complete; see setup checklist |

## Deploy

After completing `docs/superpowers/setup/2026-10-12-deployment-checklist.md`:

```bash
git push origin main          # api.yml (tests), web.yml (deploy to Cloudflare); Render + Netlify deploy from Git
```

- Migrations: embedded goose files in `apps/api/migrations`, applied by the API at startup (`RUN_MIGRATIONS=true` on Render). Locally `go run ./cmd/migrate up|status`.
- API logs: Render dashboard → `qdjr-admin-api` → Logs
- Manual admin deploy: `cd apps/web && pnpm deploy`

Domains: `api.qdjr.dev` (Render), `qdjr.dev` (Netlify), admin on `*.workers.dev`. DNS in [`infra/dns/README.md`](infra/dns/README.md).

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — context and conventions for AI-assisted development
- [`design.md`](design.md) — admin design system
- [`docs/superpowers/specs/`](docs/superpowers/specs) — design docs
- [`docs/superpowers/plans/`](docs/superpowers/plans) — implementation plans
- [`docs/superpowers/setup/`](docs/superpowers/setup) — deployment checklist

## License

Private. © Nguyen Quoc Dai.
````

- [ ] **Step 6: Commit**

```bash
git add netlify.toml infra/dns/README.md .claude/commands/deploy-api.md docs/superpowers/setup CLAUDE.md README.md
git commit -m "docs: goose migrations, Render/Cloudflare/Netlify deploy setup, DNS and checklist"
```

**Done when:**
- [ ] `netlify.toml`, new checklist, DNS README, `deploy-api.md` committed; the 2026-04-27 checklist deleted
- [ ] `CLAUDE.md` and `README.md` match steps 4–5 exactly; `grep -rn 'Cloud Run\|Vercel\|qdjr.me' CLAUDE.md README.md` prints nothing

---

### Task 12: Run everything locally and test every feature in the browser

**Files:** none (verification).

- [ ] **Step 1: Start the stack (four terminals or background jobs)**

```bash
supabase start && supabase db reset
cd apps/api
grep -q '^CORS_ORIGINS=.*3001' .env || echo 'CORS_ORIGINS=http://localhost:3000,http://localhost:3001' >> .env
env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up          # 0001–0009
env $(grep -v '^#' .env | xargs) BOOTSTRAP_ADMIN_EMAIL=admin@qdjr.local BOOTSTRAP_ADMIN_PASSWORD='ChangeMeNow123!' go run ./cmd/bootstrap
env $(grep -v '^#' .env | xargs) go run ./cmd/api                 # :8080
cd ../web && pnpm dev                                              # :3000
cd ../site && cp -n .env.example .env; API_URL=http://localhost:8080 npm run dev -- -p 3001   # :3001
curl -s localhost:8080/readyz; curl -s localhost:8080/v1/posts | head -c 200
```
Expected: `migrate up` ends at version 9; readyz `{"data":{"status":"ok"}…}`; posts envelope with `"meta":{"page":1,"perPage":20,"total":2}`. Trap: `.env` must not set `SUPABASE_JWKS_URL` locally (HS256 path) and must keep `RUN_MIGRATIONS=false` (or omit it).

- [ ] **Step 2: Automated suites**

```bash
cd apps/web && E2E_API_BASE=http://localhost:8080 pnpm exec playwright test 2>&1 | tail -4
cd apps/site && E2E_BASE_URL=http://localhost:3001 npx playwright test --project=desktop 2>&1 | tail -4
```
Expected: web `4 passed` (login, post-lifecycle, 2× profile-content); site suite passes except any screenshot-baseline tests that were already environment-dependent (report them, do not re-baseline).

- [ ] **Step 3: Browser walk-through**

Launch BrowserOS neo (`open -a "BrowserOS neo"`), then reconnect its MCP server (`/mcp` → `browseros-neo` → reconnect). If it still refuses to connect, use the Orca browser via the `orca-cli` skill (vault pref `verify-ui-in-orca-browser`; if a click returns ok without effect use `element.click()` via eval). Record one screenshot per row into `~/.claude/plans/screens/` (not the scratchpad).

| # | Where | Action | Expected |
|---|---|---|---|
| 1 | `localhost:3000/` | open | redirects to `/admin`, then to `/login` (no session) |
| 2 | `/login` | sign in `admin@qdjr.local` / `ChangeMeNow123!` | lands on `/admin`, `super_admin` badge |
| 3 | `/admin/categories` | New category `Notes` → create; edit name → `Notes 2`; delete | list updates each time; `GET /v1/categories` no longer has it |
| 4 | `/admin/tags` | same cycle with `go` | as above |
| 5 | `/admin/media` | upload a small PNG | row appears with a public URL that loads; delete it |
| 6 | `/admin/posts/new` | title `Browser test`, body `# Hi\n\nfrom neo`, category `Meta`, tag `nuxt` → Save → Publish | `GET /v1/posts/browser-test` → 200 with `published_at` |
| 7 | `localhost:3001/blog` | reload | 3 articles; `/blog/browser-test` renders `<h1>Hi</h1>`; `/en/blog/browser-test` shows the "Vietnamese only" notice |
| 8 | `/admin/posts/<id>` | Unpublish → Delete | `/blog` back to 2 |
| 9 | `/admin/profile` | change `content.vi.quote.text` → Save | `localhost:3001/` (home) shows the new quote; `localhost:3001/en` unchanged |
| 10 | `/admin/settings` | change `social_links.github` to `https://github.com/quocdaijr?tab=repositories`; remove `discord` | `localhost:3001/about` footer: GitHub link updated, Discord icon gone |
| 11 | shell | `curl -X POST localhost:8080/v1/contact -H 'Content-Type: application/json' -d '{"name":"neo","email":"neo@example.com","body":"hello"}'` | 2xx; `/admin/inbox` lists it; mark as read → status changes |
| 12 | `/admin/users` | create `editor@qdjr.local` role `editor` → change to `author` → delete | each step reflected; `/admin/users` back to 1 user |
| 12a | any admin page | press ⌘K, type `med`, Enter; then ⌘K → "Switch to dark" | lands on Media; whole UI turns dark with no white surfaces; reload keeps the theme |
| 12b | `/admin`, `/admin/posts/new` at 320px | resize | no horizontal scroll, rail collapses to the menu button, editor meta rail stacks below the writing column |
| 13 | `/admin` user menu | Sign out | back on `/login`; `/admin` redirects to `/login` |
| 14 | stop the API (Ctrl-C) → `localhost:3001/about` | reload | page still renders (bundled profile), footer still shows links; `/blog` shows "No posts yet"; no admin/API URL in page source: `curl -s localhost:3001/about \| grep -c 8080` → `0` |

- [ ] **Step 4: Restore and report**

Restart the API; revert the settings/profile edits through the admin (or `/db-reset`). Report the table with pass/fail per row and the screenshot paths. Any failure → fix in the owning task, re-run its tests, add a `fix(...)` commit.

**Done when:**
- [ ] both Playwright suites pass
- [ ] every row of the walk-through table is PASS, each with a screenshot under `~/.claude/plans/screens/`
- [ ] the DB is restored (settings/profile edits reverted)

---

### Task 13: Finish the branch

- [ ] **Step 1: Full verification**

```bash
cd apps/api && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable' go test -race ./... 2>&1 | tail -2 && cd ../..
cd apps/web && pnpm exec tsc --noEmit && pnpm lint && pnpm build 2>&1 | tail -1 && cd ../..
cd apps/site && npm run lint && npm test 2>&1 | tail -2 && npm run build 2>&1 | tail -1 && cd ../..
git status --short | wc -l
```
Expected: all green, `0` uncommitted files.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feature/dainq/goose-render-cloudflare-site-cutover
gh pr create --title "feat: goose migrations, Render/Cloudflare hosting, apps/site cutover to the API" --body-file - <<'EOF'
## Summary
- Migrations move to `apps/api/migrations` under goose, embedded in the binary and applied at API startup (`RUN_MIGRATIONS=true`); `cmd/migrate` for local use. `supabase/migrations`, `seed.sql` and `migrations.yml` are gone.
- API deploys to Render (`render.yaml`), admin to Cloudflare Workers via OpenNext (`web.yml` deploy job), public site imported as `apps/site` and built by Netlify (`netlify.toml`). GCP/Vercel config removed.
- Admin redesigned on one locked Cobalt system (`design.md`): Desk dashboard (drafts, inbox, recently published), grouped rail, ⌘K palette, light/dark, Space Grotesk + Inter + JetBrains Mono. Also fixes the self-referencing `--font-sans` that meant no web font ever loaded.
- `profile.content jsonb` (0008) + admin JSON editor; 0009 seeds posts, taxonomy, profile copy and social links from the old static site.
- `apps/site` reads posts/categories/tags/profile/social links from `/v1/*`; `@nuxt/content`, the translation pipeline and the legacy `/legacy-blogs` surface are gone.

## Test plan
- [ ] `go test -race ./...` (incl. goose contiguity + DB-backed suites), web `tsc`/lint/build, site lint/vitest/build (CI)
- [ ] Playwright: `apps/web/e2e` (login, post lifecycle, profile content) and `apps/site/e2e` against the local API
- [ ] Admin redesign: slop test + 320/375/414/768/1280 px in light and dark (Task 5A step 9)
- [ ] Browser walk-through (Task 12 table) — screenshots: <paths>
- [ ] After merge: run `docs/superpowers/setup/2026-10-12-deployment-checklist.md`; Render logs show `migrations applied`; `curl https://api.qdjr.dev/v1/posts` → 2 posts; `https://qdjr.dev/blog` renders from the API

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

- [ ] **Step 3: Hand the manual platform steps to the user**

The checklist in Task 11 step 3 is the user's to-do (Supabase pooler URL + baseline check, Render blueprint + secrets, Cloudflare token + GitHub secrets/variables, Netlify repo switch + env, DNS CNAME, prod bootstrap). Nothing in this plan can do those from the CLI without their accounts.

**Done when:**
- [ ] all three apps verify green from a clean checkout
- [ ] PR opened with the body in step 2 and the screenshot paths filled in
- [ ] the user has the Task 11 checklist as their remaining to-do

---

## Deliberately not done

- **`-- +goose Down` sections / `down` command**: the repo rule is forward-only; `migrate.Commands` refuses `down`, `reset`, `redo`. Add a Down section to a specific migration if a reversible one is ever needed.
- **A separate migrations CI job**: startup migration on Render replaces it (single instance, health-gated). If the API ever scales past one instance, move `migrate.Up` into a Render pre-deploy command (paid plans) or a job.
- **Admin custom domain** (`admin.qdjr.dev`): needs the zone on Cloudflare; user chose workers.dev for now (`infra/dns/README.md` says how later).
- **Categories menu in the site header** (`Header.vue` TODO): still `null`; `$api.listCategories()` exists if wanted.
- **Per-post author, bilingual posts, machine translation**: DB posts are single-language; English pages show the Vietnamese original with the existing notice.
- **Markdown sanitizing / syntax highlighting**: `markdown-it` only, admin-authored content is trusted (`ponytail:` comment in `markdown.ts`).
- **Site settings title/description into `<head>`**: left as the static defaults in `nuxt.config.ts`.
- **Deleting the bundled `profile.*.ts` / `SocialLinks` defaults**: kept as the offline fallback (Review Focus 1); remove once the API has uptime history.
- **Durable trip-plan cache on Netlify**: `nitro.storage.trips` stays the `fs` driver; on Netlify it is per-instance and ephemeral. Move it to Netlify Blobs (`driver: 'netlify-blobs'`) if planning latency matters.
- **A `site.yml` CI job**: Netlify runs the build; vitest for `apps/site` runs locally and in the PR checklist only.
