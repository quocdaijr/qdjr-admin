-- apps/api/migrations/0011_seed_lessons.sql
-- +goose Up
-- Suggested self-study curriculum derived from the owner's profile (expert
-- PHP/MySQL/RabbitMQ; now building on Go, Connect/gRPC, Neon, Next.js on
-- Workers). Fixed ids + on conflict do nothing: re-runnable, editable in admin.

insert into public.courses (id, title, url, provider, notes) values
  ('a0000000-0000-4000-8000-000000000001', 'gRPC & protobuf with Connect (Go)', 'https://connectrpc.com/docs/go/getting-started/', 'connectrpc.com',
   'The protocol this repo now serves lessons over. Read the proto guides first, then build mental models of the three protocols before touching streaming.'),
  ('a0000000-0000-4000-8000-000000000002', 'Go for a Laravel engineer', 'https://go.dev/doc/effective_go', 'go.dev',
   'Idioms that differ most from PHP: explicit errors, context, goroutines, table tests. Profile before optimising.'),
  ('a0000000-0000-4000-8000-000000000003', 'Postgres on Neon', 'https://neon.com/docs/introduction/architecture-overview', 'neon.com',
   'Coming from MySQL: planner, indexes and pooling behave differently; Neon adds compute suspend and branching.'),
  ('a0000000-0000-4000-8000-000000000004', 'Next.js 16 on Cloudflare Workers', 'https://nextjs.org/docs', 'nextjs.org',
   'Server vs client components, caching, and what the Workers runtime does not allow.')
on conflict (id) do nothing;

insert into public.lessons (id, course_id, position, title, url) values
  -- gRPC & protobuf with Connect
  ('b0000000-0000-4000-8000-000000000101', 'a0000000-0000-4000-8000-000000000001', 1,  'proto3 language guide',               'https://protobuf.dev/programming-guides/proto3/'),
  ('b0000000-0000-4000-8000-000000000102', 'a0000000-0000-4000-8000-000000000001', 2,  'Protobuf style guide (and buf lint)', 'https://protobuf.dev/programming-guides/style/'),
  ('b0000000-0000-4000-8000-000000000103', 'a0000000-0000-4000-8000-000000000001', 3,  'gRPC core concepts',                  'https://grpc.io/docs/what-is-grpc/core-concepts/'),
  ('b0000000-0000-4000-8000-000000000104', 'a0000000-0000-4000-8000-000000000001', 4,  'Connect protocol reference',          'https://connectrpc.com/docs/protocol/'),
  ('b0000000-0000-4000-8000-000000000105', 'a0000000-0000-4000-8000-000000000001', 5,  'Connect vs gRPC vs gRPC-Web (FAQ)',   'https://connectrpc.com/docs/faq/'),
  ('b0000000-0000-4000-8000-000000000106', 'a0000000-0000-4000-8000-000000000001', 6,  'connect-go getting started',          'https://connectrpc.com/docs/go/getting-started/'),
  ('b0000000-0000-4000-8000-000000000107', 'a0000000-0000-4000-8000-000000000001', 7,  'Interceptors',                        'https://connectrpc.com/docs/go/interceptors/'),
  ('b0000000-0000-4000-8000-000000000108', 'a0000000-0000-4000-8000-000000000001', 8,  'Errors and error details',            'https://connectrpc.com/docs/go/errors/'),
  ('b0000000-0000-4000-8000-000000000109', 'a0000000-0000-4000-8000-000000000001', 9,  'Common errors and how to debug them', 'https://connectrpc.com/docs/go/common-errors/'),
  ('b0000000-0000-4000-8000-000000000110', 'a0000000-0000-4000-8000-000000000001', 10, 'Deployment: HTTP/2, h2c, TLS',        'https://connectrpc.com/docs/go/deployment/'),
  ('b0000000-0000-4000-8000-000000000111', 'a0000000-0000-4000-8000-000000000001', 11, 'Streaming RPCs (local only on Render)', 'https://connectrpc.com/docs/go/streaming/'),
  ('b0000000-0000-4000-8000-000000000112', 'a0000000-0000-4000-8000-000000000001', 12, 'connect-es: using clients in the browser', 'https://connectrpc.com/docs/web/using-clients/'),
  ('b0000000-0000-4000-8000-000000000113', 'a0000000-0000-4000-8000-000000000001', 13, 'buf generate tutorial',               'https://buf.build/docs/generate/tutorial/'),
  ('b0000000-0000-4000-8000-000000000114', 'a0000000-0000-4000-8000-000000000001', 14, 'connect-go v2.0.0 release notes (future upgrade)', 'https://github.com/connectrpc/connect-go/releases/tag/v2.0.0'),
  -- Go for a Laravel engineer
  ('b0000000-0000-4000-8000-000000000201', 'a0000000-0000-4000-8000-000000000002', 1, 'Effective Go',                          'https://go.dev/doc/effective_go'),
  ('b0000000-0000-4000-8000-000000000202', 'a0000000-0000-4000-8000-000000000002', 2, 'Go Concurrency Patterns: Context',      'https://go.dev/blog/context'),
  ('b0000000-0000-4000-8000-000000000203', 'a0000000-0000-4000-8000-000000000002', 3, 'Pipelines and cancellation',            'https://go.dev/blog/pipelines'),
  ('b0000000-0000-4000-8000-000000000204', 'a0000000-0000-4000-8000-000000000002', 4, 'errgroup',                              'https://pkg.go.dev/golang.org/x/sync/errgroup'),
  ('b0000000-0000-4000-8000-000000000205', 'a0000000-0000-4000-8000-000000000002', 5, 'pgx v5: transactions, batches, scanning', 'https://pkg.go.dev/github.com/jackc/pgx/v5'),
  ('b0000000-0000-4000-8000-000000000206', 'a0000000-0000-4000-8000-000000000002', 6, 'The testing package: table tests, subtests, httptest', 'https://pkg.go.dev/testing'),
  ('b0000000-0000-4000-8000-000000000207', 'a0000000-0000-4000-8000-000000000002', 7, 'Profiling Go programs (pprof)',         'https://go.dev/blog/pprof'),
  ('b0000000-0000-4000-8000-000000000208', 'a0000000-0000-4000-8000-000000000002', 8, 'Go 1.27 release notes',                 'https://go.dev/doc/go1.27'),
  -- Postgres on Neon
  ('b0000000-0000-4000-8000-000000000301', 'a0000000-0000-4000-8000-000000000003', 1, 'Neon architecture: compute/storage split', 'https://neon.com/docs/introduction/architecture-overview'),
  ('b0000000-0000-4000-8000-000000000302', 'a0000000-0000-4000-8000-000000000003', 2, 'Connection pooling: pooled vs direct URLs', 'https://neon.com/docs/connect/connection-pooling'),
  ('b0000000-0000-4000-8000-000000000303', 'a0000000-0000-4000-8000-000000000003', 3, 'Branching',                             'https://neon.com/docs/introduction/branching'),
  ('b0000000-0000-4000-8000-000000000304', 'a0000000-0000-4000-8000-000000000003', 4, 'Using EXPLAIN',                         'https://www.postgresql.org/docs/current/using-explain.html'),
  ('b0000000-0000-4000-8000-000000000305', 'a0000000-0000-4000-8000-000000000003', 5, 'Index types and partial indexes',       'https://www.postgresql.org/docs/current/indexes.html'),
  ('b0000000-0000-4000-8000-000000000306', 'a0000000-0000-4000-8000-000000000003', 6, 'PostgreSQL 18 release notes',           'https://www.postgresql.org/docs/18/release-18.html'),
  -- Next.js 16 on Cloudflare Workers
  ('b0000000-0000-4000-8000-000000000401', 'a0000000-0000-4000-8000-000000000004', 1, 'Server and client components',          'https://nextjs.org/docs/app/getting-started/server-and-client-components'),
  ('b0000000-0000-4000-8000-000000000402', 'a0000000-0000-4000-8000-000000000004', 2, 'Caching and revalidating',              'https://nextjs.org/docs/app/getting-started/caching-and-revalidating'),
  ('b0000000-0000-4000-8000-000000000403', 'a0000000-0000-4000-8000-000000000004', 3, 'OpenNext Cloudflare adapter',           'https://opennext.js.org/cloudflare'),
  ('b0000000-0000-4000-8000-000000000404', 'a0000000-0000-4000-8000-000000000004', 4, 'Workers runtime limits',                'https://developers.cloudflare.com/workers/platform/limits/'),
  ('b0000000-0000-4000-8000-000000000405', 'a0000000-0000-4000-8000-000000000004', 5, 'TanStack Query: queries',               'https://tanstack.com/query/latest/docs/framework/react/guides/queries')
on conflict (id) do nothing;
