---
description: Create a new Supabase migration file
argument-hint: <name>
---

Create a new timestamped migration in `supabase/migrations/`:

```bash
supabase migration new $1
```

After editing the SQL, apply with `supabase db reset` (local) or push via the `migrations.yml` GitHub Actions workflow (remote, Plan 3).
