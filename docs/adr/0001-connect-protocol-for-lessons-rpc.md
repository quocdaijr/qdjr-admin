# Lessons admin API uses the Connect protocol, not native gRPC or REST

The owner wanted to learn gRPC on a real feature. Native gRPC needs end-to-end HTTP/2 with trailers; Render forwards one HTTP/1.1 port and states it "doesn't yet have native support for gRPC" (render.com/docs/deploy-temporal), and browsers cannot send trailers at all (connectrpc.com/docs/faq). We therefore serve `lessons.v1.LessonsService` with connect-go v1.21.0: one handler that speaks Connect, gRPC and gRPC-Web, mounted beside Gin on the same port, with h2c enabled so native gRPC works locally via `grpcurl`. The admin calls it with `@connectrpc/connect-web`. Existing `/v1/*` REST (the public site contract) is untouched.

## Considered options

- grpc-web + Envoy: a second Render service for no benefit.
- grpc-gateway: redundant, Connect already answers JSON.
- connect-go v2.0.0 (released 2026-10-07): docs and buf remote plugins still track v1; upgrade as a later lesson.

## Consequences

- Only unary RPCs in production; streaming needs end-to-end HTTP/2 (local only).
- Performance is not the reason: for a single admin on free Render + Neon the difference is unmeasurable. Learning is.
