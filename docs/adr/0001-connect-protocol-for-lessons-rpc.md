# Lessons admin API uses the Connect protocol, not native gRPC or REST

The owner wanted to learn gRPC on a real feature. Native gRPC needs end-to-end HTTP/2 with trailers; Render forwards one HTTP/1.1 port and states it "doesn't yet have native support for gRPC" (render.com/docs/deploy-temporal), and browsers cannot send trailers at all (connectrpc.com/docs/faq). We therefore serve `lessons.v1.LessonsService` with connect-go v1.21.0: one handler that speaks Connect, gRPC and gRPC-Web, mounted beside Gin on the same port, with h2c enabled so native gRPC works locally via `grpcurl`. The admin calls it with `@connectrpc/connect-web`. Existing `/v1/*` REST (the public site contract) is untouched.

## Considered options

- grpc-web + Envoy: a second Render service for no benefit.
- grpc-gateway: redundant, Connect already answers JSON.
- connect-go v2.0.0 (released 2026-10-07): docs and buf remote plugins still track v1; upgrade as a later lesson.

## Consequences

- Only unary RPCs in production; streaming needs end-to-end HTTP/2 (local only).
- Performance is not the reason: for a single admin on free Render + Neon the difference is unmeasurable. Learning is.

## Update 2026-10-13: native gRPC unary does work through Render (measured)

Lesson 7 of the gRPC course called `ListCourses` three ways against production. Native gRPC, `grpcurl -import-path proto -proto lessons/v1/lessons.proto api.qdjr.dev:443 lessons.v1.LessonsService/ListCourses`, returned the full response with a gRPC status, consistently, and the same against `qdjr-admin-api.onrender.com:443`. Render's edge (fronted by Cloudflare, `server: cloudflare`) proxies HTTP/2 end to end for unary calls, so the Render sentence cited above is at least out of date for unary RPCs.

The decision stands. Browsers still cannot send trailers, so the admin keeps using Connect over HTTP/1.1, and nothing in the stack needs native gRPC in production; it is a free extra for CLI and server-to-server callers. Streaming through Render is still unverified (lesson 17).

## Update 2026-10-13: server streaming through Render works, but idle streams are cut at 60s (measured)

`WatchCourse` (server-streaming) was run over native gRPC against `https://api.qdjr.dev` with `go run ./cmd/watch`. Watching a course that never changed, the first snapshot arrived and then the stream was reset with HTTP/2 `INTERNAL_ERROR` after exactly 60s. Watching a throwaway course that gained a lesson every 25s, the stream stayed up for 2m42s until the client closed it. So the edge (Cloudflare in front of Render) drops an HTTP/2 stream that carries no data for 60s; it does not cap stream length. `WatchCourse` therefore re-sends the unchanged snapshot after 30s of silence (`watchKeepalive`). Any future stream must send something at least that often.
