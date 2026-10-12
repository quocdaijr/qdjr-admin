# Database backups and restore

Nightly, `.github/workflows/backup.yml` runs `infra/backup/backup-db.sh`: it dumps the
production Neon database with `pg_dump` (custom format), checks the dump lists the
core tables, uploads it to the **private** R2 bucket `qdjr-backups` as
`db/qdjr-<UTC timestamp>.dump`, and keeps the newest 30.

## One-time setup

1. **Cloudflare → R2 → Create bucket** `qdjr-backups`. Leave **public access off**
   (never put dumps in the public `qdjr-media` bucket).
2. **R2 → Manage API tokens → Create Account API token**: Object Read & Write,
   *Apply to specific buckets only* → `qdjr-backups`, TTL forever. Copy the
   Access Key ID and Secret Access Key.
3. **GitHub → Settings → Secrets and variables → Actions → Secrets:**

   | Secret | Value |
   |---|---|
   | `BACKUP_DATABASE_URL` | Neon **direct** connection string (no `-pooler`), `?sslmode=require` |
   | `R2_BACKUP_ACCESS_KEY_ID` | Access Key ID from step 2 |
   | `R2_BACKUP_SECRET_ACCESS_KEY` | Secret Access Key from step 2 |

   `CLOUDFLARE_ACCOUNT_ID` is reused from the admin deploy. Optional variable
   `R2_BACKUP_BUCKET` if the bucket has another name.
4. **Actions → backup → Run workflow** once; the log ends with
   `done: db/qdjr-…dump`.

## Restore

Restore into a **new Neon branch** first, check it, then point the API at it. Never
restore over production blind.

1. Neon → Branches → **Create branch** from `main` (e.g. `restore-2026-10-12`); copy
   its direct connection string → `$TARGET`.
2. Download a dump (R2 dashboard → `qdjr-backups` → `db/` → Download), or:
   ```bash
   aws s3 cp s3://qdjr-backups/db/qdjr-<stamp>.dump ./restore.dump \
     --endpoint-url https://<account-id>.r2.cloudflarestorage.com --region auto
   ```
3. Restore (replaces objects that exist on the branch):
   ```bash
   docker run --rm -i postgres:18-alpine \
     pg_restore --clean --if-exists --no-owner --no-privileges -d "$TARGET" < ./restore.dump
   ```
4. Check: `psql "$TARGET" -c "select count(*) from posts; select max(version_id) from goose_db_version;"`
5. Switch: either make the branch Neon's primary (Branches → Set as default) or put
   its URLs into Render's `DATABASE_URL` / `MIGRATIONS_DATABASE_URL` and redeploy.

Local dry run of the whole script (Postgres + any S3-compatible mock):
`PG_DOCKER_NETWORK`, `S3_ENDPOINT` and `AWS` env vars are documented at the top of
`infra/backup/backup-db.sh`.
