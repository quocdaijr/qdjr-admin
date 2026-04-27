---
description: Start the Go API locally on :8080
---

Run the Go API from `apps/api/` against the local Supabase stack.

```bash
cd apps/api && go run ./cmd/api
```

Prereqs:
- `supabase start` is running
- `apps/api/.env` exists (copy from `.env.example`)
