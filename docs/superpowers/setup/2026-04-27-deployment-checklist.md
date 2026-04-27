# qdjr-admin deployment checklist (one-time platform setup)

This is the manual platform setup that pairs with Plan 3's CI/CD workflows. Work top-to-bottom; each section unblocks the next.

## GCP project + APIs

- [ ] Create or select a GCP project. Note `PROJECT_ID`.
- [ ] Enable APIs: Cloud Run, Artifact Registry, Secret Manager, IAM Credentials.
- [ ] Set default region: `gcloud config set run/region asia-southeast1`.

## Artifact Registry

- [ ] Create the Docker repo:
  ```bash
  gcloud artifacts repositories create qdjr \
    --repository-format=docker \
    --location=asia-southeast1
  ```

## Service account + Workload Identity Federation

- [ ] Create deployer SA: `qdjr-admin-deployer@$PROJECT.iam.gserviceaccount.com`.
- [ ] Grant roles to the SA:
  - [ ] `roles/run.admin`
  - [ ] `roles/artifactregistry.writer`
  - [ ] `roles/iam.serviceAccountUser`
  - [ ] `roles/secretmanager.secretAccessor`
- [ ] Create WIF pool: `gcloud iam workload-identity-pools create github --location=global`.
- [ ] Create OIDC provider with attribute mapping restricted to `repo:<owner>/qdjr-admin:ref:refs/heads/main`.
- [ ] Bind the SA to the pool (`roles/iam.workloadIdentityUser`).
- [ ] Capture the full provider resource name for `GCP_WORKLOAD_IDENTITY_PROVIDER`.

## GCP Secret Manager values

Create each secret (single version each):

- [ ] `DATABASE_URL` — Supabase pooled or direct connection string from Supabase dashboard. Session pooler (port 6543) is fine for serverless; direct (5432) works for Cloud Run with `min-instances=0`.
- [ ] `SUPABASE_JWT_SECRET` — Supabase legacy JWT secret (Settings → API).
- [ ] `SUPABASE_SERVICE_ROLE_KEY` — Settings → API → service_role key.
- [ ] `SUPABASE_JWKS_URL` — `https://rknqbtaybeqdzwwonlmg.supabase.co/auth/v1/.well-known/jwks.json`.

## GitHub repo secrets

Settings → Secrets and variables → Actions:

- [ ] `GCP_WORKLOAD_IDENTITY_PROVIDER` — full provider resource name.
- [ ] `GCP_SERVICE_ACCOUNT` — `qdjr-admin-deployer@$PROJECT.iam.gserviceaccount.com`.
- [ ] `GCP_PROJECT`
- [ ] `GCP_REGION` — `asia-southeast1`.
- [ ] `GCP_AR_REPO` — `qdjr`.
- [ ] `SUPABASE_URL_PROD` — `https://rknqbtaybeqdzwwonlmg.supabase.co`.
- [ ] `SUPABASE_ACCESS_TOKEN` — Supabase dashboard → Account → Access Tokens.
- [ ] `SUPABASE_DB_PASSWORD` — Supabase dashboard → Database → Settings.
- [ ] `SUPABASE_PROJECT_REF` — `rknqbtaybeqdzwwonlmg`.

## Vercel project

- [ ] Vercel dashboard → New Project → Import the GitHub repo.
- [ ] Set Root Directory to `apps/web`.
- [ ] Confirm Framework = Next.js (auto-detected). Build & install commands come from `apps/web/vercel.json`.
- [ ] Production env vars:
  - [ ] `NEXT_PUBLIC_API_URL=https://api.qdjr.me`
  - [ ] `NEXT_PUBLIC_SUPABASE_URL=https://rknqbtaybeqdzwwonlmg.supabase.co`
  - [ ] `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=<sb_publishable_*>`
- [ ] Add custom domain `admin.qdjr.me`.

## Cloud Run domain mapping

- [ ] After the first successful Cloud Run deploy, map the custom domain:
  ```bash
  gcloud beta run domain-mappings create \
    --service=qdjr-admin-api \
    --domain=api.qdjr.me \
    --region=asia-southeast1
  ```
- [ ] Apply the DNS records described in `infra/dns/README.md` at your DNS provider.

## Bootstrap super_admin in production

- [ ] Run the bootstrap CLI against prod (locally, with prod creds):
  ```bash
  cd apps/api
  DATABASE_URL=<prod direct dsn> \
  SUPABASE_URL=https://rknqbtaybeqdzwwonlmg.supabase.co \
  SUPABASE_SERVICE_ROLE_KEY=<key> \
  BOOTSTRAP_ADMIN_EMAIL=you@example.com \
  BOOTSTRAP_ADMIN_PASSWORD=<strong> \
  go run ./cmd/bootstrap
  ```
- [ ] Verify by signing in at `https://admin.qdjr.me/login` and hitting `GET /v1/admin/me` — role should be `super_admin`.
