---
description: Reset the local Postgres database and reapply all goose migrations
---

```bash
docker compose down -v db && docker compose up -d --wait db && cd apps/api && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up
```

Drops the local `db` container's volume (all data), starts a fresh Postgres 18, then applies every file in `apps/api/migrations/` in order (goose; version table `public.goose_db_version`). Restart the API afterwards: it re-creates the bootstrap admin from `BOOTSTRAP_ADMIN_*`. Never run against production.
