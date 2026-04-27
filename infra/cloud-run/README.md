# Cloud Run service config

After the first successful deploy, capture the running config for future reference:

```bash
gcloud run services describe qdjr-admin-api \
  --region=asia-southeast1 \
  --format=yaml > infra/cloud-run/qdjr-admin-api.yaml
git add infra/cloud-run/qdjr-admin-api.yaml
git commit -m "chore(infra): snapshot Cloud Run service config"
```

This isn't applied; it's a baseline for diffs and recovery.
