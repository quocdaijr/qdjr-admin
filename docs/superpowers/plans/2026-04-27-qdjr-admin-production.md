# qdjr-admin Production & Cutover — Implementation Plan (Plan 3 of 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Ship the qdjr-admin stack to production: GitHub Actions CI/CD, Google Cloud Run for the Go API, Vercel for the Next.js admin, automated Supabase migration deploys, and re-wire the qdjr public site to consume the new BE.

**Status entering this plan:** `feat/foundation` branch holds tags `v0.1.0-foundation` and `v0.2.0-resources`. All BE handlers + admin UI built and tested locally against Supabase project `rknqbtaybeqdzwwonlmg`.

**Spec:** `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md`.

---

## Two halves of this plan

**Half A (code + config)** — work I do in the repo. Tasks 1-7. All committable changes.

**Half B (manual platform setup)** — work the user does once in cloud consoles. Documented as a checklist (Task 8), not executable code.

The qdjr FE re-wire (Task 9) is code in the qdjr repo (separate working dir). It's a small diff.

---

## Half A: Code changes

### Task 1: GitHub Actions — API CI/CD

**File:** `.github/workflows/api.yml`

Trigger: push to `main`, paths `apps/api/**` or `.github/workflows/api.yml`. Also runs on PRs (test only, no deploy).

Steps for PRs and main:
1. Checkout
2. Set up Go 1.23
3. Cache Go modules
4. `go test -race ./...` (no integration tests — they need DB)
5. `golangci-lint run`

Additional steps on main only:
6. Authenticate to Google Cloud via Workload Identity Federation (OIDC, no JSON key). Use `google-github-actions/auth@v2` with `workload_identity_provider` + `service_account`.
7. Configure Docker for Artifact Registry: `gcloud auth configure-docker $REGION-docker.pkg.dev`
8. Build and push:
   ```bash
   docker build -t $REGION-docker.pkg.dev/$PROJECT/$REPO/qdjr-admin-api:$GITHUB_SHA apps/api
   docker push  $REGION-docker.pkg.dev/$PROJECT/$REPO/qdjr-admin-api:$GITHUB_SHA
   ```
9. Deploy:
   ```bash
   gcloud run deploy qdjr-admin-api \
     --image $REGION-docker.pkg.dev/$PROJECT/$REPO/qdjr-admin-api:$GITHUB_SHA \
     --region asia-southeast1 \
     --platform managed \
     --allow-unauthenticated \
     --cpu 1 --memory 256Mi --max-instances 10 \
     --set-env-vars "ENV=production,SUPABASE_URL=${{ secrets.SUPABASE_URL_PROD }}" \
     --set-secrets "DATABASE_URL=DATABASE_URL:latest,SUPABASE_JWT_SECRET=SUPABASE_JWT_SECRET:latest,SUPABASE_SERVICE_ROLE_KEY=SUPABASE_SERVICE_ROLE_KEY:latest,SUPABASE_JWKS_URL=SUPABASE_JWKS_URL:latest"
   ```
10. Smoke check: `curl -fsSL https://api.qdjr.me/healthz` (or the Cloud Run URL until DNS is wired).

Required GH secrets: `GCP_WORKLOAD_IDENTITY_PROVIDER`, `GCP_SERVICE_ACCOUNT`, `GCP_PROJECT`, `GCP_REGION`, `GCP_AR_REPO`, `SUPABASE_URL_PROD`. Plus the GCP Secret Manager values referenced by `--set-secrets`.

Commit: `chore(ci): add api workflow for build + Cloud Run deploy`.

### Task 2: GitHub Actions — Migrations

**File:** `.github/workflows/migrations.yml`

Trigger: push to `main`, paths `supabase/migrations/**` or `supabase/seed.sql`.

Steps:
1. Checkout
2. Install Supabase CLI: `curl -fsSL https://github.com/supabase/cli/releases/latest/download/supabase_linux_amd64.tar.gz | tar xz; sudo mv supabase /usr/local/bin/`
3. `supabase link --project-ref ${{ secrets.SUPABASE_PROJECT_REF }} --password ${{ secrets.SUPABASE_DB_PASSWORD }}`
4. `supabase db push --include-all` — applies any pending migrations.
5. Optional: `supabase db push --dry-run` first to log what would be applied.

Forward-only; rollback = new migration.

Required secrets: `SUPABASE_ACCESS_TOKEN` (env), `SUPABASE_DB_PASSWORD`, `SUPABASE_PROJECT_REF`.

Commit: `chore(ci): add migrations workflow for forward-only db push`.

### Task 3: GitHub Actions — Web PR checks

**File:** `.github/workflows/web.yml`

Trigger: PRs touching `apps/web/**`. Vercel auto-deploys main from its own integration, so we only run checks here.

Steps:
1. Checkout
2. Set up Node 20 + pnpm
3. Cache pnpm store
4. `pnpm install --filter ./apps/web --frozen-lockfile`
5. `cd apps/web && pnpm exec tsc --noEmit`
6. `pnpm exec next lint`
7. `pnpm build`

Commit: `chore(ci): add web PR checks`.

### Task 4: Dockerfile audit + production logger toggle

**Files:** `apps/api/Dockerfile`, `apps/api/cmd/api/main.go`

- Confirm Dockerfile uses Go 1.23, distroless base, runs as nonroot. (Already does.)
- Bump go version to 1.23 (go.mod is currently 1.25 from auto-bump — leave as-is unless Cloud Run base img needs adjustment).
- In `main.go`, when `ENV=production`, ensure `gin.SetMode(gin.ReleaseMode)` runs BEFORE `NewRouter` (already true). Also drop debug log noise: confirm `slog.NewJSONHandler` writes a single line per event and emits at INFO level by default — this is correct.

Add a `cmd/api/main.go` test for the verifier-selection branch (HS256 vs JWKS) so Plan 1's only-untested file gains some coverage. Optional; pick if quick.

Commit: `chore(api): production-mode polish`.

### Task 5: Custom domain configuration files

**Files:** `infra/cloud-run/qdjr-admin-api.yaml` (optional declarative service config), `infra/cloud-run/domain-mapping.yaml`

Generate Cloud Run YAML via `gcloud run services describe ... --format=yaml` once the service is deployed, then commit a sanitized version. This lets future deploys diff against a known-good baseline.

`infra/dns/README.md` — documents the DNS records the user needs to set:
- `api.qdjr.me` CNAME → `ghs.googlehosted.com.` (after `gcloud run domain-mappings create`)
- `admin.qdjr.me` CNAME → `cname.vercel-dns.com.`

Commit: `chore(infra): add Cloud Run service yaml and DNS docs`.

### Task 6: Update CLAUDE.md and docs/

**Files:** `CLAUDE.md`, `docs/superpowers/specs/2026-04-27-qdjr-admin-design.md` (lessons-learned section)

- CLAUDE.md: add a "Deploy" section with the three commands (`git push origin main` triggers all workflows; `gcloud run services logs read qdjr-admin-api`; Vercel CLI `vc link`).
- Spec: append a "Lessons learned" section noting:
  - Local Supabase signs JWTs with ES256 (asymmetric); JWKS path required even for local dev.
  - Next.js 16 dynamic params are `Promise<...>`.
  - shadcn defaults moved off Radix to base-ui — uses `render={...}` composition.

Commit: `docs: deploy notes + lessons learned from Plans 1-2`.

### Task 7: Vercel project config

**Files:** `apps/web/vercel.json`

```json
{
  "framework": "nextjs",
  "rootDirectory": "apps/web",
  "installCommand": "pnpm install --filter ./apps/web",
  "buildCommand": "cd apps/web && pnpm build",
  "outputDirectory": "apps/web/.next"
}
```

Plus a `.vercelignore` at repo root that excludes `apps/api/`, `supabase/`, `cmsqdjr/`, `qdjr/` from the Vercel deploy context.

Commit: `chore(web): add vercel deployment config`.

---

## Half B: Manual platform setup (Task 8 checklist)

This is what the user does once in cloud consoles. Document in `docs/superpowers/setup/2026-04-27-deployment-checklist.md`.

### GCP setup
- Create GCP project (or reuse existing). Note `PROJECT_ID`.
- Enable APIs: Cloud Run, Artifact Registry, Secret Manager, IAM.
- Create Artifact Registry repo: `gcloud artifacts repositories create qdjr --repository-format=docker --location=asia-southeast1`.
- Create service account: `qdjr-admin-deployer@$PROJECT.iam.gserviceaccount.com` with roles:
  - `roles/run.admin`
  - `roles/artifactregistry.writer`
  - `roles/iam.serviceAccountUser`
  - `roles/secretmanager.secretAccessor`
- Set up Workload Identity Federation for GitHub Actions (OIDC, no JSON key):
  - `gcloud iam workload-identity-pools create github --location=global`
  - Provider with attribute mapping for `repo:<owner>/qdjr-admin:ref:refs/heads/main`
  - Bind the service account to the pool.

### Secret Manager (in GCP)
Create secrets used by Cloud Run runtime:
- `DATABASE_URL` — Supabase pooled connection string from the Supabase dashboard (use the **session pooler** URL on port 6543 for serverless workloads; or **direct** on 5432 for Cloud Run with min-instances=0 — both work for low traffic).
- `SUPABASE_JWT_SECRET` — Supabase legacy JWT secret (Settings → API).
- `SUPABASE_SERVICE_ROLE_KEY` — Settings → API → service_role key.
- `SUPABASE_JWKS_URL` — `https://rknqbtaybeqdzwwonlmg.supabase.co/auth/v1/.well-known/jwks.json`.

### GitHub repo settings
Add repo secrets (Settings → Secrets and variables → Actions):
- `GCP_WORKLOAD_IDENTITY_PROVIDER` — full provider resource name
- `GCP_SERVICE_ACCOUNT` — `qdjr-admin-deployer@$PROJECT.iam.gserviceaccount.com`
- `GCP_PROJECT`, `GCP_REGION`, `GCP_AR_REPO`
- `SUPABASE_URL_PROD` — `https://rknqbtaybeqdzwwonlmg.supabase.co`
- `SUPABASE_ACCESS_TOKEN` — generate at Supabase dashboard → Account → Access Tokens
- `SUPABASE_DB_PASSWORD` — set/rotate at Supabase dashboard → Database → Settings
- `SUPABASE_PROJECT_REF` — `rknqbtaybeqdzwwonlmg`

### Vercel setup
- Connect Vercel to the GitHub repo (Vercel dashboard → New Project → Import).
- Set Root Directory to `apps/web`.
- Build & Output Settings: Framework = Next.js, Build Command auto-detected.
- Env vars (Production):
  - `NEXT_PUBLIC_API_URL=https://api.qdjr.me`
  - `NEXT_PUBLIC_SUPABASE_URL=https://rknqbtaybeqdzwwonlmg.supabase.co`
  - `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=<sb_publishable_*>`
- Add custom domain `admin.qdjr.me`.

### Cloud Run domain mapping
- After first deploy: `gcloud beta run domain-mappings create --service=qdjr-admin-api --domain=api.qdjr.me --region=asia-southeast1`.
- Add the CNAME record at the DNS provider (Cloudflare/Route53).

### Bootstrap super_admin in production
- SSH/locally: run the bootstrap CLI against prod:
  ```bash
  cd apps/api
  DATABASE_URL=<prod direct dsn> \
  SUPABASE_URL=https://rknqbtaybeqdzwwonlmg.supabase.co \
  SUPABASE_SERVICE_ROLE_KEY=<key> \
  BOOTSTRAP_ADMIN_EMAIL=you@example.com \
  BOOTSTRAP_ADMIN_PASSWORD=<strong> \
  go run ./cmd/bootstrap
  ```

---

## Task 9: qdjr FE re-wire

**Working dir:** `/private/var/www/html/personal/qdjr` (separate repo from qdjr-admin)

The current Nuxt site reads Markdown via `@nuxt/content`. Re-wire it to call the new BE.

### File changes
- `qdjr/nuxt.config.ts`: set `runtimeConfig.public.apiUrl = process.env.NUXT_PUBLIC_API_URL ?? 'https://api.qdjr.me'`. Remove `@nuxt/content` from modules if you want the old setup gone (or keep it as a fallback for now and switch pages over gradually).
- `qdjr/plugins/api.ts`: revive the API client. Update Post/Category/Tag interfaces to match the new BE shapes (uuid ids, `thumbnail: {url, alt}`, etc.).
- `qdjr/pages/blog/index.vue`: replace `queryCollection('blog')` with `useApi().posts.list({page, q, category, tag})`.
- `qdjr/pages/blog/[slug].vue`: replace `queryCollection` with `useApi().posts.bySlug(slug)`. Use `markdown-it` to render the Markdown content received from the API. Install: `pnpm add markdown-it @types/markdown-it`.
- `qdjr/pages/blog/tag/[tag].vue`, `qdjr/pages/blog/category/[category].vue`: switch to API calls.
- `qdjr/pages/about/index.vue`: load `/v1/profile` via `useApi().profile.get()`. Replace hardcoded content with the response.

### Migration approach
1. First PR: add the API client + types, do NOT switch pages yet. Verify the client typechecks.
2. Second PR: switch `/blog` and `/blog/[slug]` to API. Add a feature flag or env-var so you can roll back to `@nuxt/content` if needed.
3. Third PR: switch tag/category/about pages.
4. Final PR: remove `@nuxt/content` and the `content/blog/` directory.

Commit messages on the qdjr side:
- `feat(api): add typed API client for qdjr-admin BE`
- `feat(blog): switch list and post pages to API`
- `feat(blog): switch tag and category pages to API`
- `feat(about): load profile from API`
- `chore: remove @nuxt/content (migrated to API)`

### Acceptance for Task 9
- `pnpm dev` and visit `/blog`, `/blog/<existing-slug>`, `/blog/tag/<tag>`, `/blog/category/<cat>`, `/about` — all render with content from `https://api.qdjr.me`.
- Lighthouse / Web Vitals on `/blog` page within budget.
- Old Markdown files no longer needed.

---

## Verification — Plan 3 acceptance

1. `git push origin feat/foundation` triggers `api.yml` PR check workflow — passes.
2. After merge to `main`: `migrations.yml` pushes any pending schema; `api.yml` deploys Cloud Run; Vercel deploys Web.
3. `curl https://api.qdjr.me/healthz` → 200.
4. `https://admin.qdjr.me/login` loads, can sign in, see role.
5. `https://qdjr.me/blog` renders posts from the API (Task 9 done).
6. Tag `v1.0.0` on the merge commit.

## Out of scope

- Multi-region deploy
- Blue/green or canary
- Performance optimization beyond Cloud Run defaults
- Audit logs, observability dashboards (use Cloud Logging defaults)
