# DNS records for qdjr-admin

After running `gcloud beta run domain-mappings create --service=qdjr-admin-api --domain=api.qdjr.me --region=asia-southeast1`, set the records below at your DNS provider.

## api.qdjr.me

CNAME → `ghs.googlehosted.com.`

(Cloud Run will provide the exact target after `domain-mappings describe`. Replace if it differs.)

## admin.qdjr.me

CNAME → `cname.vercel-dns.com.`

## qdjr.me (root)

Unchanged — keep whatever points the public site frontend.
