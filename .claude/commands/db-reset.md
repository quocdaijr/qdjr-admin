---
description: Reset the local Supabase database, reapply all migrations and seed
---

```bash
supabase db reset
```

This drops the local DB, replays every file in `supabase/migrations/` in order, then runs `supabase/seed.sql`. Safe to run any time during development; never run against production.
