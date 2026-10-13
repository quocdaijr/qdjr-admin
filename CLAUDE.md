# qdjr-admin

Admin dashboard backend (Go/Gin) and admin UI (Next.js) for qdjr.dev, on Neon Postgres and Cloudflare R2 (the pate-shop pattern). Replaces legacy `cmsqdjr` (Laravel). Single-source-of-truth design lives in `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`.

## Tech (pinned)

- **API:** Go 1.27, Gin, `pgx/v5`, goose migrations embedded and applied at startup, deployed to Render (Docker, `render.yaml`, region singapore) at `api.qdjr.dev`
- **Web:** Next.js 16.3 (App Router, pinned for OpenNext) + shadcn/ui + TanStack Query + react-hook-form + zod, deployed to Cloudflare Workers via `@opennextjs/cloudflare` (`qdjr-admin.<account>.workers.dev`). Look is locked in `design.md` (Hallmark · Cobalt).
- **Site:** separate repo `quocdaijr/qdjr` (Nuxt 4, Netlify, `qdjr.dev`); reads posts/taxonomy/profile/settings from `/v1/*`. Keep the public API contract backward compatible.
- **Data:** Neon Postgres (pooled `DATABASE_URL` for the app, direct `MIGRATIONS_DATABASE_URL` for goose); media in Cloudflare R2 (S3 API), public URLs built from `MEDIA_PUBLIC_BASE_URL`
- **Auth:** the API's own HS256 JWTs (access 15 m / refresh 7 d) and bcrypt hashes in `public.users`; first super_admin seeded from `BOOTSTRAP_ADMIN_*` at start
- **Tooling:** goose for migrations, `docker-compose.yml` Postgres 18 for local dev and CI, GitHub Actions

## Layout

```
apps/api/              # Go backend
apps/api/migrations/   # goose SQL — single source of truth for schema (embedded in the API)
apps/web/              # Next.js admin UI
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
- **Media uploads go through the API** (multipart `POST /v1/admin/media`); the type is sniffed from the bytes (png/jpeg/webp/gif ≤ 10 MB, no SVG). Without `R2_*` (dev) files land in `apps/api/tmp/media`; production refuses to start without R2.
- **Lessons (self-study tracker) are the one admin API over Connect/gRPC**, not REST: contract in `proto/lessons/v1/lessons.proto`, served at `/lessons.v1.LessonsService/*` beside Gin on the same port, auth via `apphttp.RPCAuth` (`lessons:write`). Generated code is committed; after editing the proto run `cd apps/web && pnpm proto:gen`. h2c is on locally so `grpcurl -plaintext` works; Render only carries Connect over HTTP/1.1. See `docs/adr/0001-connect-protocol-for-lessons-rpc.md`.
- **Admin UI:** colours and fonts only through the tokens in `apps/web/app/globals.css`; no raw colour classes (`zinc-*`, `red-600`, hex). New screens follow `design.md`.

## Local dev

```bash
docker compose up -d --wait db                                                 # local Postgres 18
cd apps/api && cp .env.example .env && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up   # goose migrations
cd apps/api && go run ./cmd/api                                                # API on :8080
cd apps/web && cp .env.example .env.local && pnpm dev                          # admin on :3000
```

## See also

- Design spec: `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`
- Admin design system: `design.md`
- Active plan: `docs/superpowers/plans/2026-10-12-qdjr-admin-goose-render-cloudflare-site-cutover.md`

## Deploy

- `git push origin main` triggers: `api.yml` (test + lint; Render redeploys on its own from `render.yaml` and applies migrations at startup), `web.yml` (checks, then `wrangler deploy` to Cloudflare), `keepalive.yml` (10-min healthz ping so the free Render instance never sleeps). The public site (`quocdaijr/qdjr`) deploys separately on Netlify.
- Logs: Render dashboard → qdjr-admin-api → Logs; Cloudflare → Workers & Pages → qdjr-admin → Logs
- Manual admin deploy: `cd apps/web && pnpm run deploy` (needs `wrangler login`).
- See `infra/dns/README.md` for DNS and `docs/superpowers/setup/2026-10-12-deployment-checklist.md` for one-time platform setup.
