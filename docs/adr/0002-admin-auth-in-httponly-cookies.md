# Proposal: admin auth in httpOnly cookies, so server components can fetch

Status: proposed (2026-10-13). Not accepted; nothing implemented.

The admin keeps its access and refresh tokens in `localStorage` (`apps/web/lib/auth.ts`), and every page under `/admin` renders on the client. A spike tried TanStack Query's server prefetch on `/admin/lessons` (`prefetchQuery` in the server page, `HydrationBoundary` around the table). It cannot authenticate: on the server `localStorage` does not exist, so `getAccessToken()` returns null, the Connect call goes out with no bearer, the API answers `unauthenticated: missing bearer token` (visible in the RPC log), the interceptor's refresh fails and runs `signOut()` on the server, and `dehydrate` carries nothing. The browser then fetches as before, so the page still works, but every server render pays one wasted unauthenticated round-trip. Server-rendered admin data needs a credential the server can read: a cookie.

## What would change

- **API, `/v1/auth/login|refresh|logout`:** also set and clear the tokens as `HttpOnly; Secure` cookies (access 15 m, refresh 7 d, refresh scoped to `Path=/v1/auth`). `RequireAuth` and `RPCAuth` accept the cookie when there is no `Authorization` header, so scripts, `grpcurl` and `cmd/watch` keep working with bearers.
- **CORS:** `Access-Control-Allow-Credentials: true` for the admin origin only, in both `corsMiddleware` (Gin) and `ConnectCORS`. The public site's origins stay without credentials.
- **CSRF:** cookies are sent automatically, so every mutating call needs a guard. `SameSite=Lax` plus requiring a non-simple request (Connect already sends `Content-Type: application/json` and `Connect-Protocol-Version`, which force a preflight cross-site) covers Connect; the REST admin routes need the same rule enforced explicitly.
- **Admin (`lib/rpc.ts`, `lib/api.ts`, `lib/auth.ts`):** drop the bearer header and `localStorage`; transports use `credentials: 'include'`; server components forward the incoming cookie (`cookies()` from `next/headers`) to the API. The auth listeners become a `/v1/auth/me` check.
- **Public site (`quocdaijr/qdjr`):** unaffected; it only reads public `/v1/*`.

## The real blocker: the admin is on another site

The admin runs at `qdjr-admin.quocdaijr.workers.dev` and the API at `api.qdjr.dev`. `workers.dev` is on the public suffix list, so these are different sites: an API cookie would be third-party to the admin, needs `SameSite=None`, and is blocked by default in Safari and increasingly in Chrome. So the cookie move first needs one of:

- **A. Admin on `admin.qdjr.dev`** (Workers custom domain; DNS is on Netlify per `infra/dns/README.md`). Admin and API become same-site; cookies with `Domain=.qdjr.dev; SameSite=Lax`. Recommended: smallest moving part, keeps the browser talking to the API directly.
- **B. Backend-for-frontend in the Worker:** the browser calls same-origin routes on the Worker, which holds the token in its own first-party cookie and proxies to the API. Works on `workers.dev`, but puts the Worker in every API call (today it is in none, see the Next.js lesson 6 notes), adds a hop and Worker CPU, and duplicates the API surface.
- **C. Status quo:** `localStorage`, client-only data. No server prefetch; XSS can read the tokens (mitigated today by no third-party scripts).

## Consequences if accepted (with A)

- Server components can render admin data on first paint; the client cache hydrates from it.
- Tokens leave JavaScript's reach (httpOnly), at the cost of CSRF handling on every mutating route.
- One-time platform work: the custom domain, the `CORS_ORIGINS` and cookie `Domain` config in `render.yaml`, and a migration path that accepts both header and cookie until old tabs expire.
