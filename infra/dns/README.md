# DNS for qdjr.dev

Zone: `qdjr.dev`, registrar Hostinger, DNS hosted by Netlify (NS1 nameservers).
Manage records at app.netlify.com → Domains → qdjr.dev.

| Host | Type | Target | Serves |
|---|---|---|---|
| `qdjr.dev`, `www` | (existing) | Netlify site | public site (`quocdaijr/qdjr`) |
| `api` | CNAME | `qdjr-admin-api.onrender.com` | Go API (Render). Add the custom domain in Render → qdjr-admin-api → Settings → Custom Domains first; Render issues the certificate once the CNAME resolves. |

The admin UI has no record for now: it is served at `qdjr-admin.<account>.workers.dev`.
To move it to `admin.qdjr.dev` later, the whole zone has to be on Cloudflare
(Workers custom domains require that): change nameservers at Hostinger, recreate
the Netlify records in Cloudflare, then add `"routes": [{"pattern": "admin.qdjr.dev", "custom_domain": true}]`
to `apps/web/wrangler.jsonc` and the origin to `CORS_ORIGINS` in `render.yaml`.
