---
description: Create a new goose migration file
argument-hint: <name>
---

Create the next sequential file in `apps/api/migrations/` (forward-only, no Down section):

```bash
cd apps/api/migrations && next=$(printf '%04d' $(( 10#$(ls | grep -o '^[0-9]\{4\}' | sort | tail -1) + 1 ))) && printf -- '-- apps/api/migrations/%s_%s.sql\n-- +goose Up\n' "$next" "$1" > "${next}_$1.sql" && echo "${next}_$1.sql"
```

Wrap any statement that contains `;` inside strings or dollar-quotes in `-- +goose StatementBegin` / `-- +goose StatementEnd`. Apply locally with `/db-reset` (or `go run ./cmd/migrate up`); production applies it on the next API deploy (`RUN_MIGRATIONS=true`). `go test ./migrations/` fails on gaps or duplicate numbers.
