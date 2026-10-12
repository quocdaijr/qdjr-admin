# qdjr-admin

Admin dashboard backend (Go/Gin), admin UI (Next.js) and public site (Nuxt) for qdjr.dev, backed by Supabase. Replaces legacy `cmsqdjr` (Laravel). Single-source-of-truth design lives in `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`.

## Tech (pinned)

- **API:** Go 1.26, Gin, `pgx/v5`, goose migrations embedded and applied at startup, deployed to Render (Docker, `render.yaml`, region singapore) at `api.qdjr.dev`
- **Web:** Next.js 16.3 (App Router, pinned for OpenNext) + shadcn/ui + TanStack Query/Table + react-hook-form + zod, deployed to Cloudflare Workers via `@opennextjs/cloudflare` (`qdjr-admin.<account>.workers.dev`). Look is locked in `design.md` (Hallmark · Cobalt).
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
- Manual admin deploy: `cd apps/web && pnpm run deploy` (needs `wrangler login`).
- See `infra/dns/README.md` for DNS and `docs/superpowers/setup/2026-10-12-deployment-checklist.md` for one-time platform setup.
