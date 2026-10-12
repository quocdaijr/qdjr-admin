---
description: Deploy the API (Render redeploys from main; this is the manual path)
---

Render deploys `qdjr-admin-api` on every push to `main` (`render.yaml`, Docker build of `apps/api`), applying embedded goose migrations at startup. To redeploy by hand: Render dashboard → qdjr-admin-api → Manual Deploy → "Deploy latest commit". Check:

```bash
curl -s https://api.qdjr.dev/healthz && curl -s https://api.qdjr.dev/readyz
```
