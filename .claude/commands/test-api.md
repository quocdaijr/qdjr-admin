---
description: Run Go tests with race detector and coverage
---

```bash
cd apps/api && go test -race -cover ./...
```

For HTML coverage: `go test -race -coverprofile=coverage.out ./... && go tool cover -html=coverage.out`.
