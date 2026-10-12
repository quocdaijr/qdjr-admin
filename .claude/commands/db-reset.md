---
description: Reset the local Supabase database and reapply all goose migrations
---

```bash
supabase db reset && cd apps/api && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up
```

Drops the local DB to an empty schema, then applies every file in `apps/api/migrations/` in order (goose; version table `public.goose_db_version`). Safe any time during development; never run against production.
