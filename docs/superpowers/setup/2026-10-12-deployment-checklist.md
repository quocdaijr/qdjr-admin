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
