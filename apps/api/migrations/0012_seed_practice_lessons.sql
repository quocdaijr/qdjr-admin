-- apps/api/migrations/0012_seed_practice_lessons.sql
-- +goose Up
-- The 0011 curriculum was reading only. This adds a hands-on exercise after
-- the theory it applies, each one done in this repo, and renumbers the seed
-- lessons so the "#" column stays contiguous. Fixed ids + on conflict do
-- nothing: re-runnable, editable in admin.

insert into public.lessons (id, course_id, position, title, url, notes) values
  -- gRPC & protobuf with Connect (Go)
  ('b0000000-0000-4000-8000-000000000151', 'a0000000-0000-4000-8000-000000000001', 3,
   'Practice: break the contract on purpose', 'https://buf.build/docs/breaking/overview/',
   $n$Rename `Course.title` to `name` in `proto/lessons/v1/lessons.proto`, then run `cd apps/web && pnpm proto:gen` and `npx -y @bufbuild/buf@1.73.0 breaking --against '.git#branch=main'` from the repo root.
Read both failures: lint is about style, breaking is about wire compatibility (field numbers vs names).
Revert with `git checkout -- proto apps/api/internal/gen apps/web/gen`.
Done when: you can say which of the two checks protects a deployed client and why renaming is safe on the wire but not in generated code.$n$),
  ('b0000000-0000-4000-8000-000000000152', 'a0000000-0000-4000-8000-000000000001', 7,
   'Practice: same RPC, three protocols', 'https://github.com/fullstorydev/grpcurl',
   $n$Start the API locally and get a token from `/v1/auth/login`. Call `ListCourses` three ways and diff the headers with `-v`:
1. Connect JSON: `curl -X POST localhost:8080/lessons.v1.LessonsService/ListCourses -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{}'`
2. Connect over h2c: same with `--http2-prior-knowledge`
3. Native gRPC: `brew install grpcurl && grpcurl -plaintext -import-path proto -proto lessons/v1/lessons.proto -H "authorization: Bearer $TOKEN" localhost:8080 lessons.v1.LessonsService/ListCourses`
Then run 3 against `api.qdjr.dev:443` without `-plaintext` and watch it fail.
Done when: you can explain the failure from the ADR (Render terminates HTTP/2) without looking.$n$),
  ('b0000000-0000-4000-8000-000000000153', 'a0000000-0000-4000-8000-000000000001', 10,
   'Practice: write a logging interceptor', 'https://github.com/quocdaijr/qdjr-admin/blob/main/apps/api/internal/http/connect.go',
   $n$Add `apphttp.RPCLog(logger *slog.Logger) connect.UnaryInterceptorFunc` next to `RPCAuth`: one line per call with `req.Spec().Procedure`, the `connect.Code` (or `ok`), and duration.
TDD it in `connect_test.go` with a `slog.Handler` writing to a buffer; assert the procedure name and code appear. Mount it before `RPCAuth` in `cmd/api/main.go` so denied calls are logged too.
Done when: `go test -race ./internal/http` is green and a denied call shows `unauthenticated` in the API log.$n$),
  ('b0000000-0000-4000-8000-000000000154', 'a0000000-0000-4000-8000-000000000001', 12,
   'Practice: typed field errors', 'https://connectrpc.com/docs/go/errors/',
   $n$`cleanTitle` returns a bare `CodeInvalidArgument`. Attach a `google.rpc.BadRequest` detail (`genproto/googleapis/rpc/errdetails`) naming the field via `connErr.AddDetail(connect.NewErrorDetail(...))`.
Test it: `connect.CodeOf(err)` stays `InvalidArgument`, and `err.(*connect.Error).Details()` decodes to a `BadRequest` with `field_violations[0].field == "title"`.
In `course-dialog.tsx`, read it with `ConnectError.from(err).findDetails(BadRequestSchema)` and set the form error on that field instead of a toast.
Done when: a 201-character title shows an inline error under the Title input in the admin.$n$),
  ('b0000000-0000-4000-8000-000000000155', 'a0000000-0000-4000-8000-000000000001', 14,
   'Practice: add ReorderLessons end to end', 'https://github.com/quocdaijr/qdjr-admin/blob/main/proto/lessons/v1/lessons.proto',
   $n$New unary RPC `ReorderLessons(course_id, repeated lesson_ids) returns (lessons)`. Proto → `pnpm proto:gen` → `AdminWriter.ReorderLessons` → repository in one transaction (`pool.Begin`, one UPDATE with `unnest` + `ordinality`) → handler with validation (ids must be exactly the course's lessons) → tests at each layer.
UI: up/down buttons on each row in `course-client.tsx` calling `lessonsClient.reorderLessons`.
Done when: `go test -race ./...` and the Playwright spec pass, and the CI proto drift step is green on the PR.$n$),
  ('b0000000-0000-4000-8000-000000000156', 'a0000000-0000-4000-8000-000000000001', 17,
   'Practice: server-streaming WatchCourse over h2c', 'https://connectrpc.com/docs/go/streaming/',
   $n$Add `rpc WatchCourse(GetCourseRequest) returns (stream GetCourseResponse)`. Handler: poll the repo every 2 s and `stream.Send` when `updated_at` of the course or any lesson changed; return when `ctx.Done()`.
Write a 30-line Go client in `cmd/watch/main.go` using `connect.WithGRPC()` and an `http.Transport` with only `UnencryptedHTTP2` enabled. Flip a lesson in the admin and watch the terminal update.
Then point it at `https://api.qdjr.dev` and observe what Render does to a stream.
Done when: the local stream works and you have written one sentence in the ADR's Consequences about streaming on Render.$n$),
  ('b0000000-0000-4000-8000-000000000157', 'a0000000-0000-4000-8000-000000000001', 19,
   'Practice: call the API from a Node script', 'https://connectrpc.com/docs/node/using-clients/',
   $n$In `apps/web`, add `@connectrpc/connect-node` as a devDependency and write `scripts/list-courses.ts`: `createConnectTransport({ baseUrl, httpVersion: '1.1' })`, bearer from an env var, print `title done/total` per course. Run it with `pnpm exec tsx`.
Switch `httpVersion` to `'2'` against localhost (h2c) and against `api.qdjr.dev`; note which combinations work.
Done when: the script lists the four seed courses from production over HTTP/1.1.$n$),
  ('b0000000-0000-4000-8000-000000000158', 'a0000000-0000-4000-8000-000000000001', 21,
   'Practice: gRPC server reflection', 'https://github.com/connectrpc/grpcreflect-go',
   $n$Mount `grpcreflect.NewStaticReflector("lessons.v1.LessonsService")` handlers (v1 and v1alpha) on the outer mux in `cmd/api/main.go`, only when `APP_ENV != production`.
Now `grpcurl -plaintext localhost:8080 list` and `describe lessons.v1.LessonsService` work without `-proto`.
Done when: reflection answers locally, the production curl for `/grpc.reflection.v1.ServerReflection/ServerReflectionInfo` returns Gin's 404, and a test pins the production-off behaviour.$n$),
  ('b0000000-0000-4000-8000-000000000159', 'a0000000-0000-4000-8000-000000000001', 23,
   'Practice: upgrade to connect-go v2 on a branch', 'https://github.com/connectrpc/connect-go/releases/tag/v2.0.0',
   $n$On `chore/dainq/connect-go-v2`: bump `connectrpc.com/connect` and the `buf.build/connectrpc/go` plugin tag in `buf.gen.yaml`, regenerate, `go build ./...` and fix every compile error from the release notes.
Pay attention to interceptor ordering vs body reading (the 1 MiB `WithReadMaxBytes` cap exists because v1 reads the body before interceptors).
Done when: `go test -race ./...` is green on the branch and the PR description lists each breaking change you hit. Merge or close, your call.$n$),

  -- Go for a Laravel engineer
  ('b0000000-0000-4000-8000-000000000251', 'a0000000-0000-4000-8000-000000000002', 5,
   'Practice: run the two ListCourses queries concurrently', 'https://github.com/quocdaijr/qdjr-admin/blob/main/apps/api/internal/lessons/repository.go',
   $n$`AdminRepository.ListCourses` runs the page query then the count query. Use `errgroup.WithContext` to run both at once on the pool, return the first error, keep the existing repo tests green.
Then measure with `go test -bench` or a quick `hey -n 500` against the endpoint: is the difference visible on a local Postgres? On Neon?
Done when: tests pass under `-race` and you can say whether the change was worth it for this API.$n$),
  ('b0000000-0000-4000-8000-000000000252', 'a0000000-0000-4000-8000-000000000002', 8,
   'Practice: table-test the validators and read a coverage report', 'https://go.dev/blog/cover',
   $n$In `internal/lessons/connect_test.go` add table-driven tests for `cleanTitle` and `parseID`: empty, whitespace only, exactly 200 runes, 201 runes, 200 emoji, uppercase UUID, UUID with braces.
Run `go test -coverprofile=cover.out ./internal/lessons && go tool cover -html=cover.out` and look for red lines in the handler.
Done when: every branch of `UpdateLesson` is green in the report, or you have a test that turns it green.$n$),
  ('b0000000-0000-4000-8000-000000000253', 'a0000000-0000-4000-8000-000000000002', 10,
   'Practice: profile the API under load', 'https://pkg.go.dev/net/http/pprof',
   $n$Mount `net/http/pprof` on the outer mux at `/debug/pprof/` only when `APP_ENV != production`. Run `hey -z 30s -H "Authorization: Bearer $TOKEN" -m POST -d '{}' -T application/json localhost:8080/lessons.v1.LessonsService/ListCourses` and in parallel `go tool pprof -http=:9090 localhost:8080/debug/pprof/profile?seconds=20`.
Find the top three frames. Is it JSON, pgx, or the JWT verify?
Done when: you have a flame graph screenshot and one sentence on what you would optimise first and why you will not.$n$),

  -- Postgres on Neon
  ('b0000000-0000-4000-8000-000000000351', 'a0000000-0000-4000-8000-000000000003', 4,
   'Practice: run the migrations on a Neon branch', 'https://neon.com/docs/manage/branches',
   $n$In the Neon console create a branch `practice` from `main`. Copy its direct connection string into `MIGRATIONS_DATABASE_URL` locally and run `go run ./cmd/migrate up` and `go run ./cmd/migrate status`.
Insert a throwaway course on the branch, confirm `main` does not see it, then delete the branch.
Done when: you can explain what a branch shares with its parent (storage pages) and what it does not (compute, writes).$n$),
  ('b0000000-0000-4000-8000-000000000352', 'a0000000-0000-4000-8000-000000000003', 7,
   'Practice: EXPLAIN ANALYZE the courses list query', 'https://www.postgresql.org/docs/current/using-explain.html',
   $n$Seed load locally: `insert into public.lessons (course_id, title, position) select 'a0000000-0000-4000-8000-000000000001', 'bulk ' || g, g from generate_series(1, 200000) g;`
Run the `courseSelect` query from `repository.go` with `EXPLAIN (ANALYZE, BUFFERS)`. Note the aggregate node and whether `lessons_course_position_idx` is used.
Try `create index on public.lessons (course_id, status)` and compare. Delete the bulk rows and the index afterwards.
Done when: you have the two plans side by side and a decision on whether the index belongs in a migration.$n$),

  -- Next.js 16 on Cloudflare Workers
  ('b0000000-0000-4000-8000-000000000451', 'a0000000-0000-4000-8000-000000000004', 3,
   'Practice: try to prefetch the courses list on the server', 'https://tanstack.com/query/latest/docs/framework/react/guides/advanced-ssr',
   $n$Follow the advanced SSR guide: `prefetchQuery` in `app/admin/lessons/page.tsx`, `HydrationBoundary` around `CoursesTable`.
It will not work: the access token lives in `localStorage` (`lib/auth.ts`), which the server never sees. Trace exactly where it breaks.
Write `docs/adr/0002-...md` as a proposal only: what moving to an httpOnly cookie would change in the API (`/v1/auth/*`), the Nuxt site, and the Connect interceptor.
Done when: the branch is thrown away and the ADR proposal is in a PR for future you.$n$),
  ('b0000000-0000-4000-8000-000000000452', 'a0000000-0000-4000-8000-000000000004', 6,
   'Practice: read the Workers logs for one Connect call', 'https://developers.cloudflare.com/workers/observability/logs/workers-logs/',
   $n$Open Cloudflare → Workers & Pages → qdjr-admin → Logs, start a live tail, then flip a lesson status in the live admin.
Find the request: how much CPU time did the Worker spend, and did the Connect call to `api.qdjr.dev` go through the Worker at all or straight from the browser?
Done when: you can draw the request path on paper (browser → Worker? → Render) and know what `NEXT_PUBLIC_API_URL` being public implies.$n$),
  ('b0000000-0000-4000-8000-000000000453', 'a0000000-0000-4000-8000-000000000004', 8,
   'Practice: optimistic status updates', 'https://tanstack.com/query/latest/docs/framework/react/guides/optimistic-updates',
   $n$In `course-client.tsx`, give `statusMutation` an `onMutate` that cancels `['course', id]`, snapshots it, and writes the new status into the cache; roll back in `onError`; invalidate in `onSettled`.
Keep immutability: build a new response object, never mutate the cached message.
Test it by stopping the API, changing a status, and watching the chip revert with the error toast.
Done when: the Playwright spec still passes and the status chip changes with no visible delay on the live admin.$n$)
on conflict (id) do nothing;

-- Renumber the 0011 lessons around the new ones.
update public.lessons l set position = v.pos, updated_at = now()
from (values
  ('b0000000-0000-4000-8000-000000000101', 1),  ('b0000000-0000-4000-8000-000000000102', 2),
  ('b0000000-0000-4000-8000-000000000103', 4),  ('b0000000-0000-4000-8000-000000000104', 5),
  ('b0000000-0000-4000-8000-000000000105', 6),  ('b0000000-0000-4000-8000-000000000106', 8),
  ('b0000000-0000-4000-8000-000000000107', 9),  ('b0000000-0000-4000-8000-000000000108', 11),
  ('b0000000-0000-4000-8000-000000000109', 13), ('b0000000-0000-4000-8000-000000000110', 15),
  ('b0000000-0000-4000-8000-000000000111', 16), ('b0000000-0000-4000-8000-000000000112', 18),
  ('b0000000-0000-4000-8000-000000000113', 20), ('b0000000-0000-4000-8000-000000000114', 22),
  ('b0000000-0000-4000-8000-000000000201', 1),  ('b0000000-0000-4000-8000-000000000202', 2),
  ('b0000000-0000-4000-8000-000000000203', 3),  ('b0000000-0000-4000-8000-000000000204', 4),
  ('b0000000-0000-4000-8000-000000000205', 6),  ('b0000000-0000-4000-8000-000000000206', 7),
  ('b0000000-0000-4000-8000-000000000207', 9),  ('b0000000-0000-4000-8000-000000000208', 11),
  ('b0000000-0000-4000-8000-000000000301', 1),  ('b0000000-0000-4000-8000-000000000302', 2),
  ('b0000000-0000-4000-8000-000000000303', 3),  ('b0000000-0000-4000-8000-000000000304', 5),
  ('b0000000-0000-4000-8000-000000000305', 6),  ('b0000000-0000-4000-8000-000000000306', 8),
  ('b0000000-0000-4000-8000-000000000401', 1),  ('b0000000-0000-4000-8000-000000000402', 2),
  ('b0000000-0000-4000-8000-000000000403', 4),  ('b0000000-0000-4000-8000-000000000404', 5),
  ('b0000000-0000-4000-8000-000000000405', 7)
) as v(id, pos)
where l.id = v.id::uuid and l.position <> v.pos;
