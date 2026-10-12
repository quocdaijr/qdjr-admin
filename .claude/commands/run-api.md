---
description: Start the Go API locally on :8080
---

Run the Go API from `apps/api/` against the local Postgres container.

```bash
docker compose up -d --wait db
cd apps/api && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up && env $(grep -v '^#' .env | xargs) go run ./cmd/api
```

Prereqs:
- `apps/api/.env` exists (copy from `.env.example`, set `JWT_SECRET` with `openssl rand -hex 32`)
- Media goes to `apps/api/tmp/media` and is served at `http://localhost:8080/media-files/` unless `R2_*` are set
- The bootstrap admin (`BOOTSTRAP_ADMIN_*`) is created on first start
