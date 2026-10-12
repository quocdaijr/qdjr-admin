# Learning Lessons Tracker over Connect/gRPC — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an admin-only self-study tracker (Courses → ordered Lessons with todo / in_progress / done) whose admin API is served over the Connect protocol (gRPC-compatible) instead of REST, plus a seed of suggested courses derived from the owner's profile.

**Architecture:** The Go API keeps Gin for everything that exists today. A single generated connect-go handler for `lessons.v1.LessonsService` is mounted on an outer `http.ServeMux` in front of Gin on the same port; it speaks Connect, gRPC and gRPC-Web. Auth is one Connect interceptor (bearer JWT + `lessons:write` permission). The admin UI calls it with `@connectrpc/connect-web` through the existing TanStack Query setup. Render forwards HTTP/1.1 only and has no native gRPC ingress, so production traffic is Connect-over-HTTP/1.1 (JSON); h2c is enabled so native gRPC can be exercised locally with `grpcurl`.

**Tech Stack:** Go 1.27, Gin, pgx/v5, goose, `connectrpc.com/connect` v1.21.0, `google.golang.org/protobuf` v1.36.12, buf 1.73.0 (remote plugins `protocolbuffers/go`, `connectrpc/go`, `bufbuild/es`), Next.js 16.3.8, `@connectrpc/connect` + `@connectrpc/connect-web` 2.2.0, `@bufbuild/protobuf` 2.16.0, TanStack Query 5, react-hook-form + zod 4, Playwright.

**Spec:** the decisions in this file's "Decisions" section (grilled 2026-10-12, research cited inline). Repo-side copies to create in Task 1: `docs/superpowers/plans/2026-10-12-learning-lessons-connect.md` (this plan), `.scratch/learning-lessons/map.md` (wayfinder map), `CONTEXT.md`, `docs/adr/0001-connect-protocol-for-lessons-rpc.md`.

## Decisions (final)

| Decision | Answer |
|---|---|
| What a lesson is | Self-study tracker: **Course** (one external thing being studied) → ordered **Lessons**, status `todo`/`in_progress`/`done`, markdown notes, derived progress |
| Public exposure | Admin-only. No `/v1/lessons*`, no Nuxt change |
| gRPC scope | Lessons admin API only, via connect-go on the same port as Gin. Existing REST untouched |
| Transport | Connect over HTTP/1.1 in prod (Render: "doesn't yet have native support for gRPC", render.com/docs/deploy-temporal; browsers can't send trailers, connectrpc.com/docs/faq); h2c locally |
| connect-go major | v1.21.0 (docs/plugins line up). v2.0.0 (2026-10-07) is a later lesson |
| Codegen | `buf generate`, pinned remote plugins, generated Go + TS **committed**, CI drift check |
| Browser client | connect-web `createConnectTransport` + plain `useQuery`. No `@connectrpc/connect-query` |
| Access | Shared tracker; new permission `lessons:write` for `super_admin` and `editor`; `author` none |
| Suggestions "from my profile" | Seed migration `0011` with 4 courses (gRPC/Connect, Go for a Laravel engineer, Postgres on Neon, Next.js 16 on Workers) |
| Delivery | 3 PRs off `main`, one squash commit each: API, admin UI, seed |

Performance note (honest): for a single admin on free Render + Neon the gain is unmeasurable; the value is learning. The ADR says so.

## Global Constraints

- Response envelope / error codes rules in `CLAUDE.md` apply to REST only; Connect uses `connect.Code*` errors.
- Migrations forward-only, `-- +goose Up` only, next numbers `0010`, `0011` (`migrations_test.go` enforces contiguity).
- Immutable updates in handlers: build new values, never mutate input structs.
- Admin UI colours/fonts only through tokens in `apps/web/app/globals.css`; follow `design.md` (PageHeader + one table per list page; mono 11px uppercase for counts/dates; success silent, errors via `toast.error`; sentence case copy).
- shadcn here is base-ui: `render={<Button/>}` composition, `Select onValueChange` yields `T | null`.
- Next 16: `params` is a Promise; `await params`.
- Branch naming `<type>/dainq/<desc>` off `origin/main`; commit messages `<type>: <description>`; attribution line per the session reminder.
- Go lint: golangci v2 (`errcheck, govet, ineffassign, staticcheck, unused, revive`, gofmt, goimports). Generated files are skipped automatically (`// Code generated ... DO NOT EDIT.`).
- TDD per task; `go test -race ./...` must pass; `TEST_DATABASE_URL` set → repo tests run (local: `postgres://qdjr:qdjr@localhost:5432/qdjr?sslmode=disable` after `docker compose up -d --wait db`).
- Local Go is 1.26.5 with `GOTOOLCHAIN=auto`; `go.mod` says 1.27.0 and the toolchain auto-downloads. `http.Protocols` exists since Go 1.24.
- `buf` is not installed locally; it is run through `pnpm` (`@bufbuild/buf` devDependency) or `npx -y @bufbuild/buf@1.73.0`.

## Review Focus

1. **Whitespace-only or >200-rune titles** must be rejected with `CodeInvalidArgument` before hitting the DB `char_length` check → Task 6 tests `TestRPC_CreateCourse_BlankTitle` and `TestRPC_CreateCourse_LongTitle`.
2. **CreateLesson for a course id that does not exist** must return `CodeNotFound`, not `CodeInternal` from a FK violation → Task 4 test `TestRepo_CreateLesson_UnknownCourse_NotFound`.
3. **Marking a lesson done twice** must keep the first `completed_at`; moving off done must clear it → Task 4 test `TestRepo_UpdateLesson_DoneStampsCompletedAt`.
4. **CORS preflight from an origin not in the allowlist** must answer 204 with no `Access-Control-Allow-Origin`, and a listed origin must get the Connect headers → Task 5 tests `TestConnectCORS_PreflightAllowed` / `TestConnectCORS_PreflightUnlistedOrigin`.
5. **A refresh-token or expired token on the RPC** must yield `CodeUnauthenticated`, never a 500 → Task 6 test `TestRPC_ExpiredToken_Unauthenticated`.

## File structure

| Path | Responsibility |
|---|---|
| `proto/lessons/v1/lessons.proto` | The contract: messages + `LessonsService` (8 unary RPCs) |
| `buf.yaml`, `buf.gen.yaml` (repo root) | buf module + pinned remote plugins; outputs to both apps |
| `apps/api/internal/gen/lessons/v1/lessons.pb.go`, `.../lessonsv1connect/lessons.connect.go` | committed Go codegen |
| `apps/web/gen/lessons/v1/lessons_pb.ts` | committed TS codegen |
| `apps/api/migrations/0010_lessons.sql` | enum + `courses` + `lessons` tables |
| `apps/api/migrations/0011_seed_lessons.sql` | suggested curriculum (PR 3) |
| `apps/api/internal/rbac/static.go` (+ `static_test.go`) | `lessons:write` permission |
| `apps/api/internal/lessons/lessons.go` | domain types, inputs, `AdminWriter` interface |
| `apps/api/internal/lessons/repository.go` (+ `repository_test.go`) | pgx implementation of `AdminWriter` |
| `apps/api/internal/lessons/convert.go` | domain ↔ proto |
| `apps/api/internal/lessons/connect.go` (+ `connect_test.go`) | `Handler` implementing `LessonsServiceHandler`; validation; error mapping |
| `apps/api/internal/http/connect.go` (+ `connect_test.go`) | `ConnectCORS`, `RPCAuth` interceptor |
| `apps/api/internal/http/middleware.go` | add `UserIDFrom(ctx)` |
| `apps/api/cmd/api/main.go` | outer mux, Connect mount, h2c |
| `.github/workflows/api.yml` | proto path filters + drift check |
| `CLAUDE.md`, `CONTEXT.md`, `docs/adr/0001-...md`, `.scratch/learning-lessons/map.md` | docs |
| `apps/web/lib/rpc.ts` | transport, auth interceptor, `lessonsClient`, `rpcErrorMessage` |
| `apps/web/lib/lesson-status.ts` | enum ↔ key/label helpers shared by the lessons screens |
| `apps/web/app/admin/nav.ts` | "Learn" group |
| `apps/web/app/admin/lessons/page.tsx`, `courses-table.tsx`, `course-dialog.tsx`, `lesson-chip.tsx` | courses list |
| `apps/web/app/admin/lessons/[id]/page.tsx`, `course-client.tsx`, `lesson-dialog.tsx` | course detail + lessons |
| `apps/web/e2e/lessons.spec.ts` | end-to-end flow |
| `apps/web/package.json`, `eslint.config.mjs` | deps, `proto:gen` script, ignore `gen/**` |

---

## PR 1 — API: proto, schema, Connect service (`feature/dainq/lessons-rpc-api`)

### Task 1: Branch, docs skeleton, wayfinder map

**Files:**
- Create: `docs/superpowers/plans/2026-10-12-learning-lessons-connect.md`, `CONTEXT.md`, `docs/adr/0001-connect-protocol-for-lessons-rpc.md`, `.scratch/learning-lessons/map.md`, `.scratch/learning-lessons/issues/01-transport.md`, `02-domain-shape.md`, `03-tooling.md`

**Interfaces:** none (docs only).

- [ ] **Step 1: Branch from main**

```bash
cd /private/var/www/html/personal/qdjr-admin
git fetch origin && git checkout -b feature/dainq/lessons-rpc-api origin/main
```

- [ ] **Step 2: Copy this plan into the repo**

```bash
cp ~/.claude/plans/now-plan-and-add-jolly-wand.md docs/superpowers/plans/2026-10-12-learning-lessons-connect.md
```

- [ ] **Step 3: Write `CONTEXT.md`** (full contents)

```md
# qdjr-admin

Admin backend and UI for qdjr.dev: posts, taxonomy, media, profile, settings, inbox, users, and the owner's self-study tracker.

## Language

### Learning

**Course**:
One external thing the owner is studying — a book, a video course, a documentation set. Has a title, url, provider and notes.
_Avoid_: track, path, curriculum

**Lesson**:
One unit of a Course, ordered by position. Has a title, url, status and notes.
_Avoid_: chapter, module, item

**Lesson status**:
`todo`, `in_progress` or `done`. Setting `done` stamps the completion time; leaving `done` clears it.
_Avoid_: state, progress (that word means the derived count below)

**Progress**:
A Course's `done / total` lesson count, derived on read and never stored.
```

- [ ] **Step 4: Write the ADR** `docs/adr/0001-connect-protocol-for-lessons-rpc.md` (full contents)

```md
# Lessons admin API uses the Connect protocol, not native gRPC or REST

The owner wanted to learn gRPC on a real feature. Native gRPC needs end-to-end HTTP/2 with trailers; Render forwards one HTTP/1.1 port and states it "doesn't yet have native support for gRPC" (render.com/docs/deploy-temporal), and browsers cannot send trailers at all (connectrpc.com/docs/faq). We therefore serve `lessons.v1.LessonsService` with connect-go v1.21.0: one handler that speaks Connect, gRPC and gRPC-Web, mounted beside Gin on the same port, with h2c enabled so native gRPC works locally via `grpcurl`. The admin calls it with `@connectrpc/connect-web`. Existing `/v1/*` REST (the public site contract) is untouched.

## Considered options

- grpc-web + Envoy: a second Render service for no benefit.
- grpc-gateway: redundant, Connect already answers JSON.
- connect-go v2.0.0 (released 2026-10-07): docs and buf remote plugins still track v1; upgrade as a later lesson.

## Consequences

- Only unary RPCs in production; streaming needs end-to-end HTTP/2 (local only).
- Performance is not the reason: for a single admin on free Render + Neon the difference is unmeasurable. Learning is.
```

- [ ] **Step 5: Write the wayfinder map** `.scratch/learning-lessons/map.md`

```md
# Learning lessons tracker + Connect/gRPC

Status: charted 2026-10-12, all decision tickets resolved; execution plan at docs/superpowers/plans/2026-10-12-learning-lessons-connect.md

## Destination

An execution-ready plan for an admin-only self-study tracker served over Connect/gRPC, then three PRs implementing it.

## Notes

Domain: CONTEXT.md. Skills: superpowers:writing-plans for the plan, superpowers:test-driven-development per task. Public /v1 REST contract must stay backward compatible.

## Decisions so far

- [Transport: Connect protocol on Render](issues/01-transport.md) — Render has no native gRPC ingress; connect-go v1.21.0 beside Gin on one port, h2c locally.
- [Domain shape: Course → Lessons](issues/02-domain-shape.md) — self-study tracker, admin-only, shared, `lessons:write` for super_admin+editor.
- [Tooling: buf + committed codegen](issues/03-tooling.md) — pinned remote plugins, CI drift check, connect-web + plain useQuery.

## Not yet specified

(none — the way is clear)

## Out of scope

- Public `/v1/lessons*` and Nuxt pages.
- Converting existing REST admin routes to Connect.
- Streaming RPCs in production; connect-go v2 upgrade.
- Per-user private trackers.
```

- [ ] **Step 6: Write the three resolved tickets** (`.scratch/learning-lessons/issues/01-transport.md` shown; 02 and 03 follow the same shape with their decision and the Q/A from the grilling)

```md
# Transport for the lessons admin API

Type: research
Status: resolved

## Question

Can the admin UI (Cloudflare Workers / browser) talk native gRPC to the Go API on Render, and if not, what is the closest thing?

## Answer

No. Render: "Render doesn't yet have native support for gRPC and mTLS over external connections" (https://render.com/docs/deploy-temporal); one HTTP port per web service (https://render.com/docs/web-services). Browsers cannot send trailers (https://connectrpc.com/docs/faq). Use the Connect protocol (connect-go v1.21.0, @connectrpc/connect-web 2.2.0): unary over HTTP/1.1 works everywhere, the same handler also serves gRPC and gRPC-Web, h2c (`http.Protocols`) for local grpcurl (https://connectrpc.com/docs/go/deployment). Rejected: grpc-web+Envoy (second service), grpc-gateway (redundant), connect-go v2.0.0 (docs lag; later lesson).
```

- [ ] **Step 7: Commit**

```bash
git add docs/superpowers/plans/2026-10-12-learning-lessons-connect.md CONTEXT.md docs/adr .scratch/learning-lessons
git commit -m "docs: plan, glossary, ADR and wayfinder map for the lessons tracker over Connect"
```

### Task 2: Proto contract and codegen

**Files:**
- Create: `proto/lessons/v1/lessons.proto`, `buf.yaml`, `buf.gen.yaml`
- Create (generated): `apps/api/internal/gen/lessons/v1/lessons.pb.go`, `apps/api/internal/gen/lessons/v1/lessonsv1connect/lessons.connect.go`, `apps/web/gen/lessons/v1/lessons_pb.ts`
- Modify: `apps/web/package.json` (devDependency + script), `apps/web/eslint.config.mjs`, `apps/api/go.mod`

**Interfaces:**
- Produces Go package `lessonsv1` (`github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1`): types `Course`, `Lesson`, `LessonStatus` (consts `LessonStatus_LESSON_STATUS_UNSPECIFIED/_TODO/_IN_PROGRESS/_DONE`), `ListCoursesRequest{Page, PerPage int32; Q string}`, `ListCoursesResponse{Courses []*Course; Total int32}`, `GetCourseRequest{Id}`, `GetCourseResponse{Course *Course; Lessons []*Lesson}`, `CreateCourseRequest{Title, Url, Provider, Notes string}`, `CreateCourseResponse{Course}`, `UpdateCourseRequest{Id string; Title, Url, Provider, Notes *string}`, `UpdateCourseResponse{Course}`, `DeleteCourseRequest{Id}`, `DeleteCourseResponse{}`, `CreateLessonRequest{CourseId, Title, Url, Notes string}`, `CreateLessonResponse{Lesson}`, `UpdateLessonRequest{Id string; Title, Url, Notes *string; Status *LessonStatus; Position *int32}`, `UpdateLessonResponse{Lesson}`, `DeleteLessonRequest{Id}`, `DeleteLessonResponse{}`.
- Produces Go package `lessonsv1connect`: `LessonsServiceHandler` interface, `UnimplementedLessonsServiceHandler`, `NewLessonsServiceHandler(svc LessonsServiceHandler, opts ...connect.HandlerOption) (string, http.Handler)`, `NewLessonsServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) LessonsServiceClient`.
- Produces TS module `@/gen/lessons/v1/lessons_pb`: `LessonsService`, message types `Course`, `Lesson`, enum `LessonStatus` with members `UNSPECIFIED | TODO | IN_PROGRESS | DONE` (protobuf-es strips the `LESSON_STATUS_` prefix), request/response types with camelCase fields (`perPage`, `courseId`, `lessonCount`, `doneCount`, `completedAt?: Timestamp`).

- [ ] **Step 1: Write the proto** `proto/lessons/v1/lessons.proto` (full contents)

```proto
syntax = "proto3";

package lessons.v1;

import "google/protobuf/timestamp.proto";

option go_package = "github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1;lessonsv1";

// Self-study tracker: a Course is one external thing being studied; a Lesson
// is one ordered unit of it. Progress (done/total) is derived, never stored.

enum LessonStatus {
  LESSON_STATUS_UNSPECIFIED = 0;
  LESSON_STATUS_TODO = 1;
  LESSON_STATUS_IN_PROGRESS = 2;
  LESSON_STATUS_DONE = 3;
}

message Course {
  string id = 1;
  string title = 2;
  string url = 3;
  string provider = 4;
  string notes = 5;
  int32 lesson_count = 6;
  int32 done_count = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

message Lesson {
  string id = 1;
  string course_id = 2;
  string title = 3;
  string url = 4;
  int32 position = 5;
  LessonStatus status = 6;
  string notes = 7;
  optional google.protobuf.Timestamp completed_at = 8;
  google.protobuf.Timestamp created_at = 9;
  google.protobuf.Timestamp updated_at = 10;
}

message ListCoursesRequest {
  int32 page = 1;
  int32 per_page = 2;
  string q = 3;
}
message ListCoursesResponse {
  repeated Course courses = 1;
  int32 total = 2;
}

message GetCourseRequest { string id = 1; }
message GetCourseResponse {
  Course course = 1;
  repeated Lesson lessons = 2; // ordered by position
}

message CreateCourseRequest {
  string title = 1;
  string url = 2;
  string provider = 3;
  string notes = 4;
}
message CreateCourseResponse { Course course = 1; }

message UpdateCourseRequest {
  string id = 1;
  optional string title = 2;
  optional string url = 3;
  optional string provider = 4;
  optional string notes = 5;
}
message UpdateCourseResponse { Course course = 1; }

message DeleteCourseRequest { string id = 1; }
message DeleteCourseResponse {}

message CreateLessonRequest {
  string course_id = 1;
  string title = 2;
  string url = 3;
  string notes = 4;
}
message CreateLessonResponse { Lesson lesson = 1; }

message UpdateLessonRequest {
  string id = 1;
  optional string title = 2;
  optional string url = 3;
  optional string notes = 4;
  optional LessonStatus status = 5;
  optional int32 position = 6;
}
message UpdateLessonResponse { Lesson lesson = 1; }

message DeleteLessonRequest { string id = 1; }
message DeleteLessonResponse {}

service LessonsService {
  rpc ListCourses(ListCoursesRequest) returns (ListCoursesResponse);
  rpc GetCourse(GetCourseRequest) returns (GetCourseResponse);
  rpc CreateCourse(CreateCourseRequest) returns (CreateCourseResponse);
  rpc UpdateCourse(UpdateCourseRequest) returns (UpdateCourseResponse);
  rpc DeleteCourse(DeleteCourseRequest) returns (DeleteCourseResponse);
  rpc CreateLesson(CreateLessonRequest) returns (CreateLessonResponse);
  rpc UpdateLesson(UpdateLessonRequest) returns (UpdateLessonResponse);
  rpc DeleteLesson(DeleteLessonRequest) returns (DeleteLessonResponse);
}
```

- [ ] **Step 2: Write `buf.yaml`** (repo root, full contents)

```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

- [ ] **Step 3: Write `buf.gen.yaml`** (repo root, full contents). Before committing, open https://buf.build/protocolbuffers/go, https://buf.build/connectrpc/go and https://buf.build/bufbuild/es in a browser and confirm the three tags exist; if `protocolbuffers/go:v1.36.12` is absent, use `v1.36.11` here **and** `go get google.golang.org/protobuf@v1.36.11` in Step 6.

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go:v1.36.12
    out: apps/api/internal/gen
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go:v1.18.1
    out: apps/api/internal/gen
    opt: paths=source_relative
  - remote: buf.build/bufbuild/es:v2.16.0
    out: apps/web/gen
    opt: target=ts
```

- [ ] **Step 4: Add buf to the web workspace and a script**

```bash
cd apps/web && pnpm add -D @bufbuild/buf@1.73.0
```

Edit `apps/web/package.json` scripts — FIND:

```json
    "test:e2e": "playwright test"
```

REPLACE:

```json
    "test:e2e": "playwright test",
    "proto:gen": "cd ../.. && buf lint && buf generate"
```

- [ ] **Step 5: Generate and inspect**

```bash
cd apps/web && pnpm proto:gen
ls ../../apps/api/internal/gen/lessons/v1 ../../apps/api/internal/gen/lessons/v1/lessonsv1connect gen/lessons/v1
```
Expected: `lessons.pb.go`, `lessons.connect.go`, `lessons_pb.ts`; `buf lint` prints nothing.

- [ ] **Step 6: Go dependencies**

```bash
cd apps/api && go get connectrpc.com/connect@v1.21.0 google.golang.org/protobuf@v1.36.12 && go mod tidy && go build ./...
```
Expected: builds; `go.mod` now lists both as direct requirements.

- [ ] **Step 7: ESLint ignores generated TS** — edit `apps/web/eslint.config.mjs`, FIND:

```js
    ".open-next/**",
    ".wrangler/**",
  ]),
```

REPLACE:

```js
    ".open-next/**",
    ".wrangler/**",
    // buf-generated protobuf/connect code
    "gen/**",
  ]),
```

- [ ] **Step 8: Verify the web side type-checks with the generated module**

```bash
cd apps/web && pnpm exec tsc --noEmit && pnpm lint
```
Expected: clean.

- [ ] **Step 9: Commit**

```bash
git add proto buf.yaml buf.gen.yaml apps/api/internal/gen apps/api/go.mod apps/api/go.sum apps/web/gen apps/web/package.json apps/web/pnpm-lock.yaml apps/web/eslint.config.mjs
git commit -m "feat(api): lessons.v1 proto contract with buf codegen for Go and TS"
```

### Task 3: Permission and schema

**Files:**
- Modify: `apps/api/internal/rbac/static.go:12-32`
- Test: `apps/api/internal/rbac/static_test.go`
- Create: `apps/api/migrations/0010_lessons.sql`

**Interfaces:**
- Produces permission string `"lessons:write"`; tables `public.courses`, `public.lessons`, enum `public.lesson_status`.

- [ ] **Step 1: Write the failing permission test** — append to `apps/api/internal/rbac/static_test.go`:

```go
func TestStatic_Can_LessonsWrite(t *testing.T) {
	for role, want := range map[string]bool{"super_admin": true, "editor": true, "author": false} {
		u := uuid.New()
		r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: role}})
		ok, err := r.Can(context.Background(), u, "lessons:write")
		require.NoError(t, err)
		assert.Equal(t, want, ok, role)
	}
}
```

- [ ] **Step 2: Run it, expect failure**

```bash
cd apps/api && go test ./internal/rbac -run TestStatic_Can_LessonsWrite -v
```
Expected: FAIL for `super_admin` and `editor` (`false != true`).

- [ ] **Step 3: Add the permission** — `apps/api/internal/rbac/static.go`, FIND:

```go
		"contact:read", "contact:write",
		"users:manage",
	},
	"editor": {
		"posts:read:all", "posts:write", "posts:publish",
		"taxonomy:write",
		"media:write",
		"profile:write", "settings:write",
		"contact:read", "contact:write",
	},
```

REPLACE:

```go
		"contact:read", "contact:write",
		"lessons:write",
		"users:manage",
	},
	"editor": {
		"posts:read:all", "posts:write", "posts:publish",
		"taxonomy:write",
		"media:write",
		"profile:write", "settings:write",
		"contact:read", "contact:write",
		"lessons:write",
	},
```

- [ ] **Step 4: Run, expect pass**

```bash
go test ./internal/rbac -v
```

- [ ] **Step 5: Write the migration** `apps/api/migrations/0010_lessons.sql` (full contents)

```sql
-- apps/api/migrations/0010_lessons.sql
-- +goose Up
-- Self-study tracker: a course is one external thing being studied, a lesson
-- one ordered unit of it. Progress is derived from lessons.status, never stored.

create type public.lesson_status as enum ('todo', 'in_progress', 'done');

create table public.courses (
    id          uuid primary key default gen_random_uuid(),
    title       text not null check (char_length(title) between 1 and 200),
    url         text not null default '',
    provider    text not null default '',
    notes       text not null default '',
    created_by  uuid references public.users(id) on delete set null,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

-- Lessons have no life outside their course: cascade is intentional here.
create table public.lessons (
    id           uuid primary key default gen_random_uuid(),
    course_id    uuid not null references public.courses(id) on delete cascade,
    title        text not null check (char_length(title) between 1 and 200),
    url          text not null default '',
    position     integer not null default 0,
    status       public.lesson_status not null default 'todo',
    notes        text not null default '',
    completed_at timestamptz,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now()
);

create index lessons_course_position_idx on public.lessons (course_id, position);
```

- [ ] **Step 6: Apply locally and run the migrations test**

```bash
docker compose up -d --wait db
cd apps/api && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up
go test ./migrations -v
psql postgres://qdjr:qdjr@localhost:5432/qdjr -c '\d public.lessons'
```
Expected: migration `0010` applied; contiguity test passes; table shows `status | lesson_status | not null | default 'todo'`.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/rbac apps/api/migrations/0010_lessons.sql
git commit -m "feat(api): lessons schema and lessons:write permission"
```

### Task 4: Domain types and pgx repository

**Files:**
- Create: `apps/api/internal/lessons/lessons.go`, `apps/api/internal/lessons/repository.go`
- Test: `apps/api/internal/lessons/repository_test.go`

**Interfaces:**
- Produces package `lessons`: `Status` (`StatusTodo`, `StatusInProgress`, `StatusDone`), `ValidStatus(Status) bool`, `ErrNotFound`, `Course`, `Lesson`, `ListFilter`, `CreateCourseInput`, `UpdateCourseInput`, `CreateLessonInput`, `UpdateLessonInput`, interface `AdminWriter`, `NewAdminRepository(*pgxpool.Pool) *AdminRepository`.

- [ ] **Step 1: Write `lessons.go`** (full contents)

```go
// Package lessons is the self-study tracker: Courses with ordered Lessons. It
// is the one admin resource served over Connect/gRPC instead of REST (see
// docs/adr/0001-connect-protocol-for-lessons-rpc.md).
package lessons

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status is a lesson's progress state (public.lesson_status).
type Status string

// The three lesson states.
const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// ValidStatus reports whether s is one of the known states.
func ValidStatus(s Status) bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

// ErrNotFound is returned when no course or lesson matches the id.
var ErrNotFound = errors.New("lessons: not found")

// Course is one external thing being studied. LessonCount and DoneCount are
// derived from its lessons by the repository and never stored.
type Course struct {
	ID          uuid.UUID
	Title       string
	URL         string
	Provider    string
	Notes       string
	LessonCount int
	DoneCount   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Lesson is one unit of a Course, ordered by Position.
type Lesson struct {
	ID          uuid.UUID
	CourseID    uuid.UUID
	Title       string
	URL         string
	Position    int
	Status      Status
	Notes       string
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListFilter narrows ListCourses. Q is an ILIKE on title.
type ListFilter struct {
	Page    int
	PerPage int
	Q       string
}

// CreateCourseInput is the payload for CreateCourse. CreatedBy may be
// uuid.Nil (stored as NULL).
type CreateCourseInput struct {
	Title     string
	URL       string
	Provider  string
	Notes     string
	CreatedBy uuid.UUID
}

// UpdateCourseInput is a partial update; nil means "do not change".
type UpdateCourseInput struct {
	Title    *string
	URL      *string
	Provider *string
	Notes    *string
}

// CreateLessonInput appends a lesson at the end of the course.
type CreateLessonInput struct {
	CourseID uuid.UUID
	Title    string
	URL      string
	Notes    string
}

// UpdateLessonInput is a partial update; nil means "do not change". Setting
// Status to StatusDone stamps CompletedAt (kept if already set); any other
// status clears it.
type UpdateLessonInput struct {
	Title    *string
	URL      *string
	Notes    *string
	Status   *Status
	Position *int
}

// AdminWriter is the contract the Connect handler depends on. An interface so
// handler tests run against an in-memory stub.
type AdminWriter interface {
	ListCourses(ctx context.Context, f ListFilter) ([]Course, int, error)
	GetCourse(ctx context.Context, id uuid.UUID) (Course, []Lesson, error)
	CreateCourse(ctx context.Context, in CreateCourseInput) (Course, error)
	UpdateCourse(ctx context.Context, id uuid.UUID, in UpdateCourseInput) (Course, error)
	DeleteCourse(ctx context.Context, id uuid.UUID) error
	CreateLesson(ctx context.Context, in CreateLessonInput) (Lesson, error)
	UpdateLesson(ctx context.Context, id uuid.UUID, in UpdateLessonInput) (Lesson, error)
	DeleteLesson(ctx context.Context, id uuid.UUID) error
}
```

- [ ] **Step 2: Write the failing repository tests** `apps/api/internal/lessons/repository_test.go` (full contents)

```go
package lessons

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	t       *testing.T
	pool    *pgxpool.Pool
	ctx     context.Context
	repo    *AdminRepository
	courses []uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	f := &fixture{t: t, pool: pool, ctx: ctx, repo: NewAdminRepository(pool)}
	t.Cleanup(func() {
		for _, id := range f.courses {
			_, _ = pool.Exec(context.Background(), `delete from public.courses where id = $1`, id)
		}
	})
	return f
}

func (f *fixture) course(title string) Course {
	f.t.Helper()
	c, err := f.repo.CreateCourse(f.ctx, CreateCourseInput{Title: title, URL: "https://example.test", Provider: "test"})
	require.NoError(f.t, err)
	f.courses = append(f.courses, c.ID)
	return c
}

func TestRepo_CreateCourse_AndGet_ZeroProgress(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-create-" + uuid.NewString()[:8])

	got, lessons, err := f.repo.GetCourse(f.ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.Title, got.Title)
	assert.Equal(t, "test", got.Provider)
	assert.Equal(t, 0, got.LessonCount)
	assert.Equal(t, 0, got.DoneCount)
	assert.Empty(t, lessons)
}

func TestRepo_GetCourse_Unknown_NotFound(t *testing.T) {
	f := newFixture(t)
	_, _, err := f.repo.GetCourse(f.ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_CreateLesson_AppendsPosition(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-pos-" + uuid.NewString()[:8])

	l1, err := f.repo.CreateLesson(f.ctx, CreateLessonInput{CourseID: c.ID, Title: "one"})
	require.NoError(t, err)
	l2, err := f.repo.CreateLesson(f.ctx, CreateLessonInput{CourseID: c.ID, Title: "two"})
	require.NoError(t, err)

	assert.Equal(t, 1, l1.Position)
	assert.Equal(t, 2, l2.Position)
	assert.Equal(t, StatusTodo, l1.Status)
	assert.Nil(t, l1.CompletedAt)

	_, lessons, err := f.repo.GetCourse(f.ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, lessons, 2)
	assert.Equal(t, "one", lessons[0].Title)
	assert.Equal(t, "two", lessons[1].Title)
}

func TestRepo_CreateLesson_UnknownCourse_NotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.repo.CreateLesson(f.ctx, CreateLessonInput{CourseID: uuid.New(), Title: "orphan"})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_UpdateLesson_DoneStampsCompletedAt(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-done-" + uuid.NewString()[:8])
	l, err := f.repo.CreateLesson(f.ctx, CreateLessonInput{CourseID: c.ID, Title: "finish me"})
	require.NoError(t, err)

	done := StatusDone
	first, err := f.repo.UpdateLesson(f.ctx, l.ID, UpdateLessonInput{Status: &done})
	require.NoError(t, err)
	require.NotNil(t, first.CompletedAt)

	// Done again keeps the original stamp.
	again, err := f.repo.UpdateLesson(f.ctx, l.ID, UpdateLessonInput{Status: &done})
	require.NoError(t, err)
	require.NotNil(t, again.CompletedAt)
	assert.True(t, first.CompletedAt.Equal(*again.CompletedAt))

	course, _, err := f.repo.GetCourse(f.ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, course.LessonCount)
	assert.Equal(t, 1, course.DoneCount)

	// Leaving done clears it.
	todo := StatusTodo
	back, err := f.repo.UpdateLesson(f.ctx, l.ID, UpdateLessonInput{Status: &todo})
	require.NoError(t, err)
	assert.Nil(t, back.CompletedAt)
}

func TestRepo_UpdateLesson_Unknown_NotFound(t *testing.T) {
	f := newFixture(t)
	title := "x"
	_, err := f.repo.UpdateLesson(f.ctx, uuid.New(), UpdateLessonInput{Title: &title})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_UpdateCourse_Partial(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-upd-" + uuid.NewString()[:8])
	provider := "go.dev"
	got, err := f.repo.UpdateCourse(f.ctx, c.ID, UpdateCourseInput{Provider: &provider})
	require.NoError(t, err)
	assert.Equal(t, "go.dev", got.Provider)
	assert.Equal(t, c.Title, got.Title)
}

func TestRepo_ListCourses_FilterByTitle(t *testing.T) {
	f := newFixture(t)
	needle := "repo-list-" + uuid.NewString()[:8]
	f.course(needle + " alpha")
	f.course("unrelated " + uuid.NewString()[:8])

	out, total, err := f.repo.ListCourses(f.ctx, ListFilter{Page: 1, PerPage: 10, Q: needle})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, out, 1)
	assert.Contains(t, out[0].Title, needle)
}

func TestRepo_DeleteCourse_Cascades(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-del-" + uuid.NewString()[:8])
	l, err := f.repo.CreateLesson(f.ctx, CreateLessonInput{CourseID: c.ID, Title: "gone"})
	require.NoError(t, err)

	require.NoError(t, f.repo.DeleteCourse(f.ctx, c.ID))
	_, _, err = f.repo.GetCourse(f.ctx, c.ID)
	assert.ErrorIs(t, err, ErrNotFound)
	var n int
	require.NoError(t, f.pool.QueryRow(f.ctx, `select count(*) from public.lessons where id = $1`, l.ID).Scan(&n))
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, f.repo.DeleteCourse(f.ctx, c.ID), ErrNotFound)
}

func TestRepo_DeleteLesson(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-dell-" + uuid.NewString()[:8])
	l, err := f.repo.CreateLesson(f.ctx, CreateLessonInput{CourseID: c.ID, Title: "bye"})
	require.NoError(t, err)
	require.NoError(t, f.repo.DeleteLesson(f.ctx, l.ID))
	assert.ErrorIs(t, f.repo.DeleteLesson(f.ctx, l.ID), ErrNotFound)
}
```

- [ ] **Step 3: Run, expect compile failure**

```bash
cd apps/api && TEST_DATABASE_URL=postgres://qdjr:qdjr@localhost:5432/qdjr?sslmode=disable go test ./internal/lessons -run TestRepo -v
```
Expected: FAIL — `undefined: AdminRepository`, `NewAdminRepository`.

- [ ] **Step 4: Write `repository.go`** (full contents)

```go
package lessons

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pagination bounds shared with the Connect handler.
const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// AdminRepository is the pgx-backed AdminWriter.
type AdminRepository struct{ pool *pgxpool.Pool }

// NewAdminRepository constructs an AdminRepository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository { return &AdminRepository{pool: pool} }

// courseSelect joins lesson counts so progress is derived in one query.
// Callers append WHERE ... GROUP BY c.id.
const courseSelect = `
    SELECT c.id, c.title, c.url, c.provider, c.notes,
           count(l.id)::int,
           (count(l.id) FILTER (WHERE l.status = 'done'))::int,
           c.created_at, c.updated_at
    FROM public.courses c
    LEFT JOIN public.lessons l ON l.course_id = c.id`

const lessonColumns = `id, course_id, title, url, position, status::text, notes, completed_at, created_at, updated_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanCourse(s rowScanner) (Course, error) {
	var c Course
	err := s.Scan(&c.ID, &c.Title, &c.URL, &c.Provider, &c.Notes,
		&c.LessonCount, &c.DoneCount, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func scanLesson(s rowScanner) (Lesson, error) {
	var l Lesson
	var status string
	err := s.Scan(&l.ID, &l.CourseID, &l.Title, &l.URL, &l.Position, &status,
		&l.Notes, &l.CompletedAt, &l.CreatedAt, &l.UpdatedAt)
	l.Status = Status(status)
	return l, err
}

// ListCourses returns a page of courses (newest activity first) with derived
// progress, filtered by title when f.Q is non-empty.
func (r *AdminRepository) ListCourses(ctx context.Context, f ListFilter) ([]Course, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = defaultPerPage
	}
	if f.PerPage > maxPerPage {
		f.PerPage = maxPerPage
	}
	q := strings.TrimSpace(f.Q)

	const listSQL = courseSelect + `
        WHERE ($1::text = '' OR c.title ILIKE '%' || $1 || '%')
        GROUP BY c.id
        ORDER BY c.updated_at DESC
        LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, listSQL, q, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("lessons list courses: %w", err)
	}
	defer rows.Close()
	out := make([]Course, 0, f.PerPage)
	for rows.Next() {
		c, err := scanCourse(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("lessons list courses scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("lessons list courses iterate: %w", err)
	}

	const countSQL = `SELECT count(*) FROM public.courses WHERE ($1::text = '' OR title ILIKE '%' || $1 || '%')`
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, q).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("lessons count courses: %w", err)
	}
	return out, total, nil
}

// GetCourse returns the course with derived progress and its lessons ordered
// by position, or ErrNotFound.
func (r *AdminRepository) GetCourse(ctx context.Context, id uuid.UUID) (Course, []Lesson, error) {
	c, err := scanCourse(r.pool.QueryRow(ctx, courseSelect+` WHERE c.id = $1 GROUP BY c.id`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Course{}, nil, ErrNotFound
	}
	if err != nil {
		return Course{}, nil, fmt.Errorf("lessons get course: %w", err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+lessonColumns+` FROM public.lessons WHERE course_id = $1 ORDER BY position, created_at`, id)
	if err != nil {
		return Course{}, nil, fmt.Errorf("lessons get course lessons: %w", err)
	}
	defer rows.Close()
	lessons := make([]Lesson, 0, c.LessonCount)
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return Course{}, nil, fmt.Errorf("lessons get course scan: %w", err)
		}
		lessons = append(lessons, l)
	}
	if err := rows.Err(); err != nil {
		return Course{}, nil, fmt.Errorf("lessons get course iterate: %w", err)
	}
	return c, lessons, nil
}

// CreateCourse inserts a course and returns it with zero progress.
func (r *AdminRepository) CreateCourse(ctx context.Context, in CreateCourseInput) (Course, error) {
	id := uuid.New()
	const insert = `
        INSERT INTO public.courses (id, title, url, provider, notes, created_by)
        VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := r.pool.Exec(ctx, insert, id, in.Title, in.URL, in.Provider, in.Notes, nullableUUID(in.CreatedBy)); err != nil {
		return Course{}, fmt.Errorf("lessons insert course: %w", err)
	}
	c, _, err := r.GetCourse(ctx, id)
	return c, err
}

// UpdateCourse applies a partial update. Returns ErrNotFound if no row matches.
func (r *AdminRepository) UpdateCourse(ctx context.Context, id uuid.UUID, in UpdateCourseInput) (Course, error) {
	b := newSetBuilder()
	if in.Title != nil {
		b.add("title", *in.Title)
	}
	if in.URL != nil {
		b.add("url", *in.URL)
	}
	if in.Provider != nil {
		b.add("provider", *in.Provider)
	}
	if in.Notes != nil {
		b.add("notes", *in.Notes)
	}
	if err := r.execUpdate(ctx, "public.courses", b, id); err != nil {
		return Course{}, err
	}
	c, _, err := r.GetCourse(ctx, id)
	return c, err
}

// DeleteCourse removes a course and, by cascade, its lessons.
func (r *AdminRepository) DeleteCourse(ctx context.Context, id uuid.UUID) error {
	return r.execDelete(ctx, "public.courses", id)
}

// CreateLesson appends a lesson after the course's last position.
func (r *AdminRepository) CreateLesson(ctx context.Context, in CreateLessonInput) (Lesson, error) {
	id := uuid.New()
	const insert = `
        INSERT INTO public.lessons (id, course_id, title, url, notes, position)
        VALUES ($1, $2, $3, $4, $5,
                (SELECT coalesce(max(position), 0) + 1 FROM public.lessons WHERE course_id = $2))`
	if _, err := r.pool.Exec(ctx, insert, id, in.CourseID, in.Title, in.URL, in.Notes); err != nil {
		if isForeignKeyViolation(err) {
			return Lesson{}, ErrNotFound
		}
		return Lesson{}, fmt.Errorf("lessons insert lesson: %w", err)
	}
	return r.getLesson(ctx, id)
}

// UpdateLesson applies a partial update; see UpdateLessonInput for the
// completed_at rule. Returns ErrNotFound if no row matches.
func (r *AdminRepository) UpdateLesson(ctx context.Context, id uuid.UUID, in UpdateLessonInput) (Lesson, error) {
	b := newSetBuilder()
	if in.Title != nil {
		b.add("title", *in.Title)
	}
	if in.URL != nil {
		b.add("url", *in.URL)
	}
	if in.Notes != nil {
		b.add("notes", *in.Notes)
	}
	if in.Position != nil {
		b.add("position", *in.Position)
	}
	if in.Status != nil {
		b.addCast("status", "public.lesson_status", string(*in.Status))
		if *in.Status == StatusDone {
			b.raw("completed_at = coalesce(completed_at, now())")
		} else {
			b.raw("completed_at = NULL")
		}
	}
	if err := r.execUpdate(ctx, "public.lessons", b, id); err != nil {
		return Lesson{}, err
	}
	return r.getLesson(ctx, id)
}

// DeleteLesson removes one lesson.
func (r *AdminRepository) DeleteLesson(ctx context.Context, id uuid.UUID) error {
	return r.execDelete(ctx, "public.lessons", id)
}

func (r *AdminRepository) getLesson(ctx context.Context, id uuid.UUID) (Lesson, error) {
	l, err := scanLesson(r.pool.QueryRow(ctx, `SELECT `+lessonColumns+` FROM public.lessons WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Lesson{}, ErrNotFound
	}
	if err != nil {
		return Lesson{}, fmt.Errorf("lessons get lesson: %w", err)
	}
	return l, nil
}

// execUpdate runs UPDATE <table> SET ... , updated_at = now() WHERE id = $n.
// A builder with no columns is a no-op that still checks existence.
func (r *AdminRepository) execUpdate(ctx context.Context, table string, b *setBuilder, id uuid.UUID) error {
	b.raw("updated_at = now()")
	stmt := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d`, table, strings.Join(b.sets, ", "), b.next)
	args := append(b.args, id)
	cmd, err := r.pool.Exec(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("lessons update %s: %w", table, err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AdminRepository) execDelete(ctx context.Context, table string, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, table), id)
	if err != nil {
		return fmt.Errorf("lessons delete %s: %w", table, err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// setBuilder accumulates "col = $n" fragments for a dynamic UPDATE, the same
// shape tags/repository_admin.go builds inline.
type setBuilder struct {
	sets []string
	args []any
	next int
}

func newSetBuilder() *setBuilder { return &setBuilder{next: 1} }

func (b *setBuilder) add(col string, v any) {
	b.sets = append(b.sets, fmt.Sprintf("%s = $%d", col, b.next))
	b.args = append(b.args, v)
	b.next++
}

func (b *setBuilder) addCast(col, pgType string, v any) {
	b.sets = append(b.sets, fmt.Sprintf("%s = $%d::%s", col, b.next, pgType))
	b.args = append(b.args, v)
	b.next++
}

func (b *setBuilder) raw(fragment string) { b.sets = append(b.sets, fragment) }

func nullableUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

// isForeignKeyViolation reports Postgres error 23503.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
```

- [ ] **Step 5: Run the repository tests, expect pass**

```bash
cd apps/api && TEST_DATABASE_URL=postgres://qdjr:qdjr@localhost:5432/qdjr?sslmode=disable go test ./internal/lessons -run TestRepo -v
```
Expected: all `TestRepo_*` PASS.

- [ ] **Step 6: Lint and commit**

```bash
cd apps/api && gofmt -l ./internal/lessons && go vet ./internal/lessons
git add apps/api/internal/lessons/lessons.go apps/api/internal/lessons/repository.go apps/api/internal/lessons/repository_test.go
git commit -m "feat(api): lessons domain types and pgx repository"
```

### Task 5: Connect CORS and auth interceptor in package http

**Files:**
- Create: `apps/api/internal/http/connect.go`
- Modify: `apps/api/internal/http/middleware.go:68-73`
- Test: `apps/api/internal/http/connect_test.go`

**Interfaces:**
- Produces `apphttp.ConnectCORS(allowed []string) func(http.Handler) http.Handler`, `apphttp.RPCAuth(v auth.Verifier, res rbac.PermissionResolver, perm string) connect.UnaryInterceptorFunc`, `apphttp.UserIDFrom(ctx context.Context) (uuid.UUID, bool)`.
- Consumes `auth.Verifier` (`Verify(token string) (auth.Claims, error)`), `rbac.PermissionResolver.Can`, `rbac.ErrNoRole`.

- [ ] **Step 1: Write the failing tests** `apps/api/internal/http/connect_test.go` (full contents)

```go
package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

const connectTestSecret = "connect-secret-connect-secret-1234"

type permStub struct {
	perms map[string]bool
	err   error
}

func (p *permStub) Role(context.Context, uuid.UUID) (string, error) { return "editor", p.err }
func (p *permStub) Permissions(context.Context, uuid.UUID) ([]string, error) {
	return nil, p.err
}
func (p *permStub) Can(_ context.Context, _ uuid.UUID, perm string) (bool, error) {
	return p.perms[perm], p.err
}

func signedToken(t *testing.T, uid uuid.UUID, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uid.String(),
		"exp": exp.Unix(),
	})
	s, err := tok.SignedString([]byte(connectTestSecret))
	require.NoError(t, err)
	return s
}

// callThrough runs the interceptor with a next that records the ctx user id.
func callThrough(t *testing.T, i connect.UnaryInterceptorFunc, authHeader string) (uuid.UUID, bool, error) {
	t.Helper()
	var seen uuid.UUID
	var ok bool
	next := func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		seen, ok = UserIDFrom(ctx)
		return connect.NewResponse(&struct{}{}), nil
	}
	req := connect.NewRequest(&struct{}{})
	if authHeader != "" {
		req.Header().Set("Authorization", authHeader)
	}
	_, err := i(next)(context.Background(), req)
	return seen, ok, err
}

func TestRPCAuth_MissingToken_Unauthenticated(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{"lessons:write": true}}, "lessons:write")
	_, _, err := callThrough(t, i, "")
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPCAuth_ExpiredToken_Unauthenticated(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{"lessons:write": true}}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(-time.Minute)))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPCAuth_MissingPermission_PermissionDenied(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{}}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(time.Hour)))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPCAuth_NoRole_PermissionDenied(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{err: rbac.ErrNoRole}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(time.Hour)))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPCAuth_ResolverError_Internal(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{err: errors.New("db down")}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(time.Hour)))
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestRPCAuth_OK_PutsUserInContext(t *testing.T) {
	uid := uuid.New()
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{"lessons:write": true}}, "lessons:write")
	seen, ok, err := callThrough(t, i, "Bearer "+signedToken(t, uid, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, uid, seen)
}

func TestConnectCORS_PreflightAllowed(t *testing.T) {
	h := ConnectCORS([]string{"https://admin.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/lessons.v1.LessonsService/ListCourses", nil)
	req.Header.Set("Origin", "https://admin.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "https://admin.example", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Connect-Protocol-Version")
	assert.Equal(t, "Origin", rr.Header().Get("Vary"))
}

func TestConnectCORS_PreflightUnlistedOrigin(t *testing.T) {
	h := ConnectCORS([]string{"https://admin.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/lessons.v1.LessonsService/ListCourses", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestConnectCORS_PassesPostThrough(t *testing.T) {
	h := ConnectCORS([]string{"https://admin.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodPost, "/lessons.v1.LessonsService/ListCourses", nil)
	req.Header.Set("Origin", "https://admin.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTeapot, rr.Code)
	assert.Equal(t, "https://admin.example", rr.Header().Get("Access-Control-Allow-Origin"))
}
```

- [ ] **Step 2: Run, expect compile failure**

```bash
cd apps/api && go test ./internal/http -run 'TestRPCAuth|TestConnectCORS' -v
```
Expected: FAIL — `undefined: RPCAuth`, `ConnectCORS`, `UserIDFrom`.

- [ ] **Step 3: Add `UserIDFrom`** — `apps/api/internal/http/middleware.go`, FIND:

```go
// UserIDFromContext returns the authenticated caller's id (set by RequireAuth).
func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v := c.Request.Context().Value(ctxUserID)
	id, ok := v.(uuid.UUID)
	return id, ok
}
```

REPLACE:

```go
// UserIDFromContext returns the authenticated caller's id (set by RequireAuth).
func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	return UserIDFrom(c.Request.Context())
}

// UserIDFrom reads the caller id stored by RequireAuth (Gin) or RPCAuth
// (Connect) from a plain context.
func UserIDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxUserID).(uuid.UUID)
	return id, ok
}
```

- [ ] **Step 4: Write `connect.go`** (full contents)

```go
package http

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"connectrpc.com/connect"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// ConnectCORS is the net/http twin of corsMiddleware for the Connect mount,
// which sits in front of Gin on the outer mux. connect-web sends
// Connect-Protocol-Version on every call and Connect-Timeout-Ms when a
// timeout is set; nothing it reads back needs Access-Control-Expose-Headers.
// Preflights always get 204, listed origins also get the allow headers.
func ConnectCORS(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "POST,OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Connect-Protocol-Version,Connect-Timeout-Ms")
				h.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RPCAuth verifies the bearer token and the permission in one unary
// interceptor so every protocol the handler speaks (Connect, gRPC, gRPC-Web)
// gets a proper connect.Error instead of a JSON 401 body. connect-go v1 runs
// interceptors after unmarshal; fine for the small admin payloads here.
func RPCAuth(v auth.Verifier, res rbac.PermissionResolver, perm string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			h := req.Header().Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
			}
			claims, err := v.Verify(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			ok, err := res.Can(ctx, claims.UserID, perm)
			switch {
			case errors.Is(err, rbac.ErrNoRole):
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("no role assigned"))
			case err != nil:
				return nil, connect.NewError(connect.CodeInternal, err)
			case !ok:
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("missing permission: "+perm))
			}
			return next(context.WithValue(ctx, ctxUserID, claims.UserID), req)
		}
	}
}
```

- [ ] **Step 5: Run, expect pass**

```bash
cd apps/api && go test ./internal/http -v
```
Expected: the new tests and the existing `router_test.go` all PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/http/connect.go apps/api/internal/http/connect_test.go apps/api/internal/http/middleware.go
git commit -m "feat(api): Connect CORS middleware and bearer+permission RPC interceptor"
```

### Task 6: Connect handler for LessonsService

**Files:**
- Create: `apps/api/internal/lessons/convert.go`, `apps/api/internal/lessons/connect.go`
- Test: `apps/api/internal/lessons/connect_test.go`

**Interfaces:**
- Consumes `AdminWriter` (Task 4), `lessonsv1`/`lessonsv1connect` (Task 2), `apphttp.RPCAuth`, `apphttp.ConnectCORS`, `apphttp.UserIDFrom` (Task 5).
- Produces `lessons.NewHandler(repo AdminWriter) *Handler` where `*Handler` implements `lessonsv1connect.LessonsServiceHandler`.

- [ ] **Step 1: Write `convert.go`** (full contents)

```go
package lessons

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	lessonsv1 "github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1"
)

func statusToProto(s Status) lessonsv1.LessonStatus {
	switch s {
	case StatusTodo:
		return lessonsv1.LessonStatus_LESSON_STATUS_TODO
	case StatusInProgress:
		return lessonsv1.LessonStatus_LESSON_STATUS_IN_PROGRESS
	case StatusDone:
		return lessonsv1.LessonStatus_LESSON_STATUS_DONE
	}
	return lessonsv1.LessonStatus_LESSON_STATUS_UNSPECIFIED
}

// statusFromProto maps the wire enum; UNSPECIFIED and unknown values are rejected.
func statusFromProto(s lessonsv1.LessonStatus) (Status, bool) {
	switch s {
	case lessonsv1.LessonStatus_LESSON_STATUS_TODO:
		return StatusTodo, true
	case lessonsv1.LessonStatus_LESSON_STATUS_IN_PROGRESS:
		return StatusInProgress, true
	case lessonsv1.LessonStatus_LESSON_STATUS_DONE:
		return StatusDone, true
	}
	return "", false
}

func courseToProto(c Course) *lessonsv1.Course {
	return &lessonsv1.Course{
		Id:          c.ID.String(),
		Title:       c.Title,
		Url:         c.URL,
		Provider:    c.Provider,
		Notes:       c.Notes,
		LessonCount: int32(c.LessonCount),
		DoneCount:   int32(c.DoneCount),
		CreatedAt:   timestamppb.New(c.CreatedAt),
		UpdatedAt:   timestamppb.New(c.UpdatedAt),
	}
}

func lessonToProto(l Lesson) *lessonsv1.Lesson {
	return &lessonsv1.Lesson{
		Id:          l.ID.String(),
		CourseId:    l.CourseID.String(),
		Title:       l.Title,
		Url:         l.URL,
		Position:    int32(l.Position),
		Status:      statusToProto(l.Status),
		Notes:       l.Notes,
		CompletedAt: optionalTimestamp(l.CompletedAt),
		CreatedAt:   timestamppb.New(l.CreatedAt),
		UpdatedAt:   timestamppb.New(l.UpdatedAt),
	}
}

func optionalTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
```

- [ ] **Step 2: Write the failing handler tests** `apps/api/internal/lessons/connect_test.go` (full contents)

```go
package lessons

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	lessonsv1 "github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1/lessonsv1connect"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

const testSecret = "lessons-secret-lessons-secret-1234"

// resolverStub is a minimal PermissionResolver double.
type resolverStub struct{ perms map[string]bool }

func (r *resolverStub) Role(context.Context, uuid.UUID) (string, error) { return "editor", nil }
func (r *resolverStub) Permissions(context.Context, uuid.UUID) ([]string, error) {
	out := make([]string, 0, len(r.perms))
	for k := range r.perms {
		out = append(out, k)
	}
	return out, nil
}
func (r *resolverStub) Can(_ context.Context, _ uuid.UUID, p string) (bool, error) {
	return r.perms[p], nil
}

// stubRepo is an in-memory AdminWriter that records the last inputs.
type stubRepo struct {
	courses        map[uuid.UUID]Course
	lessons        map[uuid.UUID]Lesson
	listFilt       ListFilter
	createCourseIn CreateCourseInput
	updateLessonIn UpdateLessonInput
}

func newStubRepo() *stubRepo {
	return &stubRepo{courses: map[uuid.UUID]Course{}, lessons: map[uuid.UUID]Lesson{}}
}

func (s *stubRepo) ListCourses(_ context.Context, f ListFilter) ([]Course, int, error) {
	s.listFilt = f
	out := make([]Course, 0, len(s.courses))
	for _, c := range s.courses {
		out = append(out, c)
	}
	return out, len(out), nil
}

func (s *stubRepo) GetCourse(_ context.Context, id uuid.UUID) (Course, []Lesson, error) {
	c, ok := s.courses[id]
	if !ok {
		return Course{}, nil, ErrNotFound
	}
	var ls []Lesson
	for _, l := range s.lessons {
		if l.CourseID == id {
			ls = append(ls, l)
		}
	}
	return c, ls, nil
}

func (s *stubRepo) CreateCourse(_ context.Context, in CreateCourseInput) (Course, error) {
	s.createCourseIn = in
	now := time.Now().UTC()
	c := Course{ID: uuid.New(), Title: in.Title, URL: in.URL, Provider: in.Provider, Notes: in.Notes, CreatedAt: now, UpdatedAt: now}
	s.courses[c.ID] = c
	return c, nil
}

func (s *stubRepo) UpdateCourse(_ context.Context, id uuid.UUID, in UpdateCourseInput) (Course, error) {
	c, ok := s.courses[id]
	if !ok {
		return Course{}, ErrNotFound
	}
	if in.Title != nil {
		c.Title = *in.Title
	}
	if in.URL != nil {
		c.URL = *in.URL
	}
	if in.Provider != nil {
		c.Provider = *in.Provider
	}
	if in.Notes != nil {
		c.Notes = *in.Notes
	}
	s.courses[id] = c
	return c, nil
}

func (s *stubRepo) DeleteCourse(_ context.Context, id uuid.UUID) error {
	if _, ok := s.courses[id]; !ok {
		return ErrNotFound
	}
	delete(s.courses, id)
	return nil
}

func (s *stubRepo) CreateLesson(_ context.Context, in CreateLessonInput) (Lesson, error) {
	if _, ok := s.courses[in.CourseID]; !ok {
		return Lesson{}, ErrNotFound
	}
	now := time.Now().UTC()
	l := Lesson{ID: uuid.New(), CourseID: in.CourseID, Title: in.Title, URL: in.URL, Notes: in.Notes, Position: len(s.lessons) + 1, Status: StatusTodo, CreatedAt: now, UpdatedAt: now}
	s.lessons[l.ID] = l
	return l, nil
}

func (s *stubRepo) UpdateLesson(_ context.Context, id uuid.UUID, in UpdateLessonInput) (Lesson, error) {
	s.updateLessonIn = in
	l, ok := s.lessons[id]
	if !ok {
		return Lesson{}, ErrNotFound
	}
	if in.Title != nil {
		l.Title = *in.Title
	}
	if in.Status != nil {
		l.Status = *in.Status
		if *in.Status == StatusDone {
			now := time.Now().UTC()
			l.CompletedAt = &now
		} else {
			l.CompletedAt = nil
		}
	}
	s.lessons[id] = l
	return l, nil
}

func (s *stubRepo) DeleteLesson(_ context.Context, id uuid.UUID) error {
	if _, ok := s.lessons[id]; !ok {
		return ErrNotFound
	}
	delete(s.lessons, id)
	return nil
}

type env struct {
	srv  *httptest.Server
	repo *stubRepo
}

// newEnv builds the mux exactly like cmd/api/main.go does: Connect handler
// behind ConnectCORS + RPCAuth, h2c enabled.
func newEnv(t *testing.T, perms ...string) *env {
	t.Helper()
	repo := newStubRepo()
	res := &resolverStub{perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(testSecret))
	mux := http.NewServeMux()
	path, h := lessonsv1connect.NewLessonsServiceHandler(
		NewHandler(repo),
		connect.WithInterceptors(apphttp.RPCAuth(v, res, "lessons:write")),
	)
	mux.Handle(path, apphttp.ConnectCORS(nil)(h))

	srv := httptest.NewUnstartedServer(mux)
	var p http.Protocols
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = &p
	srv.Start()
	t.Cleanup(srv.Close)
	return &env{srv: srv, repo: repo}
}

func token(t *testing.T, uid uuid.UUID, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": uid.String(), "exp": exp.Unix()})
	s, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

func bearer(tok string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+tok)
			return next(ctx, req)
		}
	}))
}

func (e *env) client(t *testing.T, uid uuid.UUID, opts ...connect.ClientOption) lessonsv1connect.LessonsServiceClient {
	t.Helper()
	opts = append([]connect.ClientOption{bearer(token(t, uid, time.Now().Add(time.Hour)))}, opts...)
	return lessonsv1connect.NewLessonsServiceClient(http.DefaultClient, e.srv.URL, opts...)
}

func TestRPC_NoToken_Unauthenticated(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := lessonsv1connect.NewLessonsServiceClient(http.DefaultClient, e.srv.URL)
	_, err := c.ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPC_ExpiredToken_Unauthenticated(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := lessonsv1connect.NewLessonsServiceClient(http.DefaultClient, e.srv.URL, bearer(token(t, uuid.New(), time.Now().Add(-time.Minute))))
	_, err := c.ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPC_MissingPermission_PermissionDenied(t *testing.T) {
	e := newEnv(t) // author-like: no lessons:write
	_, err := e.client(t, uuid.New()).ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPC_ListCourses_PassesFilter(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{Page: 2, PerPage: 5, Q: "go"}))
	require.NoError(t, err)
	assert.Equal(t, ListFilter{Page: 2, PerPage: 5, Q: "go"}, e.repo.listFilt)
}

func TestRPC_CreateCourse_OK_RecordsCreator(t *testing.T) {
	e := newEnv(t, "lessons:write")
	uid := uuid.New()
	res, err := e.client(t, uid).CreateCourse(context.Background(), connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: "  Effective Go  ", Url: "https://go.dev/doc/effective_go", Provider: "go.dev"}))
	require.NoError(t, err)
	assert.Equal(t, "Effective Go", res.Msg.GetCourse().GetTitle())
	assert.Equal(t, uid, e.repo.createCourseIn.CreatedBy)
	assert.NotNil(t, res.Msg.GetCourse().GetCreatedAt())
}

func TestRPC_CreateCourse_BlankTitle_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(), connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: "   "}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_CreateCourse_LongTitle_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	// 201 multibyte runes: byte length would be 603, rune count is what matters.
	long := strings.Repeat("é", 201)
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(), connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: long}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_GetCourse_BadID_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).GetCourse(context.Background(), connect.NewRequest(&lessonsv1.GetCourseRequest{Id: "nope"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_GetCourse_Unknown_NotFound(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).GetCourse(context.Background(), connect.NewRequest(&lessonsv1.GetCourseRequest{Id: uuid.NewString()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRPC_UpdateLesson_StatusDone(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	course, err := c.CreateCourse(context.Background(), connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: "c"}))
	require.NoError(t, err)
	lesson, err := c.CreateLesson(context.Background(), connect.NewRequest(&lessonsv1.CreateLessonRequest{CourseId: course.Msg.GetCourse().GetId(), Title: "l"}))
	require.NoError(t, err)

	done := lessonsv1.LessonStatus_LESSON_STATUS_DONE
	res, err := c.UpdateLesson(context.Background(), connect.NewRequest(&lessonsv1.UpdateLessonRequest{Id: lesson.Msg.GetLesson().GetId(), Status: &done}))
	require.NoError(t, err)
	assert.Equal(t, lessonsv1.LessonStatus_LESSON_STATUS_DONE, res.Msg.GetLesson().GetStatus())
	assert.NotNil(t, res.Msg.GetLesson().GetCompletedAt())
	require.NotNil(t, e.repo.updateLessonIn.Status)
	assert.Equal(t, StatusDone, *e.repo.updateLessonIn.Status)
}

func TestRPC_UpdateLesson_UnspecifiedStatus_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	unspecified := lessonsv1.LessonStatus_LESSON_STATUS_UNSPECIFIED
	_, err := e.client(t, uuid.New()).UpdateLesson(context.Background(), connect.NewRequest(&lessonsv1.UpdateLessonRequest{Id: uuid.NewString(), Status: &unspecified}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_DeleteCourse_Unknown_NotFound(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).DeleteCourse(context.Background(), connect.NewRequest(&lessonsv1.DeleteCourseRequest{Id: uuid.NewString()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestRPC_NativeGRPC_OverH2C proves the same handler answers real gRPC when
// the client speaks prior-knowledge HTTP/2 (what grpcurl -plaintext does).
func TestRPC_NativeGRPC_OverH2C(t *testing.T) {
	e := newEnv(t, "lessons:write")
	var cp http.Protocols
	cp.SetUnencryptedHTTP2(true) // no HTTP/1 → prior-knowledge h2c
	h2c := &http.Client{Transport: &http.Transport{Protocols: &cp}}
	c := lessonsv1connect.NewLessonsServiceClient(h2c, e.srv.URL,
		connect.WithGRPC(), bearer(token(t, uuid.New(), time.Now().Add(time.Hour))))
	res, err := c.ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{}))
	require.NoError(t, err)
	assert.Equal(t, int32(0), res.Msg.GetTotal())
}
```

- [ ] **Step 3: Run, expect compile failure**

```bash
cd apps/api && go test ./internal/lessons -run TestRPC -v
```
Expected: FAIL — `undefined: NewHandler`.

- [ ] **Step 4: Write `connect.go`** (full contents)

```go
package lessons

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	lessonsv1 "github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1/lessonsv1connect"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

const maxTitleRunes = 200

// Handler implements lessonsv1connect.LessonsServiceHandler on top of an
// AdminWriter. Auth and permission are enforced by the RPCAuth interceptor
// registered in cmd/api/main.go, so every method can assume a caller.
type Handler struct {
	lessonsv1connect.UnimplementedLessonsServiceHandler
	repo AdminWriter
}

var _ lessonsv1connect.LessonsServiceHandler = (*Handler)(nil)

// NewHandler constructs a Handler.
func NewHandler(repo AdminWriter) *Handler { return &Handler{repo: repo} }

// ListCourses returns a page of courses with derived progress.
func (h *Handler) ListCourses(ctx context.Context, req *connect.Request[lessonsv1.ListCoursesRequest]) (*connect.Response[lessonsv1.ListCoursesResponse], error) {
	courses, total, err := h.repo.ListCourses(ctx, ListFilter{
		Page:    int(req.Msg.GetPage()),
		PerPage: int(req.Msg.GetPerPage()),
		Q:       req.Msg.GetQ(),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*lessonsv1.Course, 0, len(courses))
	for _, c := range courses {
		out = append(out, courseToProto(c))
	}
	return connect.NewResponse(&lessonsv1.ListCoursesResponse{Courses: out, Total: int32(total)}), nil
}

// GetCourse returns one course and its lessons ordered by position.
func (h *Handler) GetCourse(ctx context.Context, req *connect.Request[lessonsv1.GetCourseRequest]) (*connect.Response[lessonsv1.GetCourseResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	c, lessons, err := h.repo.GetCourse(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*lessonsv1.Lesson, 0, len(lessons))
	for _, l := range lessons {
		out = append(out, lessonToProto(l))
	}
	return connect.NewResponse(&lessonsv1.GetCourseResponse{Course: courseToProto(c), Lessons: out}), nil
}

// CreateCourse validates the title and records the caller as creator.
func (h *Handler) CreateCourse(ctx context.Context, req *connect.Request[lessonsv1.CreateCourseRequest]) (*connect.Response[lessonsv1.CreateCourseResponse], error) {
	title, err := cleanTitle(req.Msg.GetTitle())
	if err != nil {
		return nil, err
	}
	uid, _ := apphttp.UserIDFrom(ctx)
	c, err := h.repo.CreateCourse(ctx, CreateCourseInput{
		Title:     title,
		URL:       strings.TrimSpace(req.Msg.GetUrl()),
		Provider:  strings.TrimSpace(req.Msg.GetProvider()),
		Notes:     req.Msg.GetNotes(),
		CreatedBy: uid,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&lessonsv1.CreateCourseResponse{Course: courseToProto(c)}), nil
}

// UpdateCourse applies the optional fields that were set.
func (h *Handler) UpdateCourse(ctx context.Context, req *connect.Request[lessonsv1.UpdateCourseRequest]) (*connect.Response[lessonsv1.UpdateCourseResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	in := UpdateCourseInput{
		URL:      trimmed(req.Msg.Url),
		Provider: trimmed(req.Msg.Provider),
		Notes:    req.Msg.Notes,
	}
	if req.Msg.Title != nil {
		title, err := cleanTitle(*req.Msg.Title)
		if err != nil {
			return nil, err
		}
		in.Title = &title
	}
	c, err := h.repo.UpdateCourse(ctx, id, in)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&lessonsv1.UpdateCourseResponse{Course: courseToProto(c)}), nil
}

// DeleteCourse removes a course and its lessons.
func (h *Handler) DeleteCourse(ctx context.Context, req *connect.Request[lessonsv1.DeleteCourseRequest]) (*connect.Response[lessonsv1.DeleteCourseResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.repo.DeleteCourse(ctx, id); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&lessonsv1.DeleteCourseResponse{}), nil
}

// CreateLesson appends a lesson to a course.
func (h *Handler) CreateLesson(ctx context.Context, req *connect.Request[lessonsv1.CreateLessonRequest]) (*connect.Response[lessonsv1.CreateLessonResponse], error) {
	courseID, err := parseID(req.Msg.GetCourseId())
	if err != nil {
		return nil, err
	}
	title, err := cleanTitle(req.Msg.GetTitle())
	if err != nil {
		return nil, err
	}
	l, err := h.repo.CreateLesson(ctx, CreateLessonInput{
		CourseID: courseID,
		Title:    title,
		URL:      strings.TrimSpace(req.Msg.GetUrl()),
		Notes:    req.Msg.GetNotes(),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&lessonsv1.CreateLessonResponse{Lesson: lessonToProto(l)}), nil
}

// UpdateLesson applies the optional fields that were set; status drives completed_at.
func (h *Handler) UpdateLesson(ctx context.Context, req *connect.Request[lessonsv1.UpdateLessonRequest]) (*connect.Response[lessonsv1.UpdateLessonResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	in := UpdateLessonInput{
		URL:   trimmed(req.Msg.Url),
		Notes: req.Msg.Notes,
	}
	if req.Msg.Title != nil {
		title, err := cleanTitle(*req.Msg.Title)
		if err != nil {
			return nil, err
		}
		in.Title = &title
	}
	if req.Msg.Status != nil {
		st, ok := statusFromProto(*req.Msg.Status)
		if !ok {
			return nil, invalid("status must be todo, in_progress or done")
		}
		in.Status = &st
	}
	if req.Msg.Position != nil {
		if *req.Msg.Position < 0 {
			return nil, invalid("position must be >= 0")
		}
		pos := int(*req.Msg.Position)
		in.Position = &pos
	}
	l, err := h.repo.UpdateLesson(ctx, id, in)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&lessonsv1.UpdateLessonResponse{Lesson: lessonToProto(l)}), nil
}

// DeleteLesson removes one lesson.
func (h *Handler) DeleteLesson(ctx context.Context, req *connect.Request[lessonsv1.DeleteLessonRequest]) (*connect.Response[lessonsv1.DeleteLessonResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.repo.DeleteLesson(ctx, id); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&lessonsv1.DeleteLessonResponse{}), nil
}

// --- helpers ---

func invalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

func mapErr(err error) error {
	if errors.Is(err, ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, invalid("invalid id")
	}
	return id, nil
}

// cleanTitle trims and enforces 1..maxTitleRunes characters (the DB check
// uses char_length, so count runes, not bytes).
func cleanTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > maxTitleRunes {
		return "", invalid(fmt.Sprintf("title must be 1..%d characters", maxTitleRunes))
	}
	return s, nil
}

// trimmed returns a new pointer to the trimmed value, or nil when unset.
func trimmed(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}
```

- [ ] **Step 5: Run, expect pass**

```bash
cd apps/api && go test ./internal/lessons -run TestRPC -v
```
Expected: all `TestRPC_*` PASS, including `TestRPC_NativeGRPC_OverH2C`.

- [ ] **Step 6: Full package tests + lint, commit**

```bash
cd apps/api && TEST_DATABASE_URL=postgres://qdjr:qdjr@localhost:5432/qdjr?sslmode=disable go test -race ./internal/lessons && gofmt -l ./internal/lessons
git add apps/api/internal/lessons
git commit -m "feat(api): Connect handler for lessons.v1.LessonsService"
```

### Task 7: Mount on the server, CI drift check, docs

**Files:**
- Modify: `apps/api/cmd/api/main.go:17-33` (imports), `:111` (repo), `:159-163` (server)
- Modify: `.github/workflows/api.yml`
- Modify: `CLAUDE.md` (Tech + Local dev)

**Interfaces:**
- Consumes `lessons.NewAdminRepository`, `lessons.NewHandler`, `lessonsv1connect.NewLessonsServiceHandler`, `apphttp.RPCAuth`, `apphttp.ConnectCORS`.

- [ ] **Step 1: Imports** — `apps/api/cmd/api/main.go`, FIND:

```go
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/adminapi"
```

REPLACE:

```go
	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/adminapi"
```

FIND:

```go
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/db"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/media"
```

REPLACE:

```go
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/db"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1/lessonsv1connect"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/lessons"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/media"
```

- [ ] **Step 2: Repository** — FIND:

```go
	usersRepo := users.NewAdminRepository(pool)
	accounts := users.NewPGAccounts(pool)
```

REPLACE:

```go
	usersRepo := users.NewAdminRepository(pool)
	accounts := users.NewPGAccounts(pool)
	lessonsRepo := lessons.NewAdminRepository(pool)
```

- [ ] **Step 3: Server** — FIND:

```go
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
```

REPLACE:

```go
	// Lessons is served over Connect (gRPC-compatible) on the same port. The
	// outer mux sends the service prefix to Connect and everything else to Gin,
	// so Gin's CORS and routes are untouched. Auth lives in the RPCAuth
	// interceptor so gRPC and gRPC-Web callers get proper status codes.
	mux := http.NewServeMux()
	lessonsPath, lessonsHandler := lessonsv1connect.NewLessonsServiceHandler(
		lessons.NewHandler(lessonsRepo),
		connect.WithInterceptors(apphttp.RPCAuth(issuer, resolver, "lessons:write")),
	)
	mux.Handle(lessonsPath, apphttp.ConnectCORS(cfg.CORSOrigins)(lessonsHandler))
	mux.Handle("/", r)

	// h2c lets native gRPC clients (grpcurl -plaintext) reach the handler
	// locally. Render terminates TLS and forwards HTTP/1.1, where Connect's
	// unary RPCs still work.
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		Protocols:         &protocols,
	}
```

- [ ] **Step 4: Build and smoke locally**

```bash
cd apps/api && go build ./... && go vet ./... && (env $(grep -v '^#' .env | xargs) go run ./cmd/api &) && sleep 3
TOKEN=$(curl -s -X POST localhost:8080/v1/auth/login -H 'Content-Type: application/json' \
  -d "{\"email\":\"$(grep BOOTSTRAP_ADMIN_EMAIL .env | cut -d= -f2)\",\"password\":\"$(grep BOOTSTRAP_ADMIN_PASSWORD .env | cut -d= -f2)\"}" | jq -r .data.access_token)
curl -s -X POST localhost:8080/lessons.v1.LessonsService/ListCourses \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{}'
curl -s localhost:8080/v1/posts | head -c 120; echo
curl -s -X POST localhost:8080/lessons.v1.LessonsService/ListCourses -H 'Content-Type: application/json' -d '{}'
```
Expected, in order: `{"total":0}` or `{}` (Connect JSON omits zero values); the REST envelope `{"data":[...` still served by Gin; `{"code":"unauthenticated","message":"missing bearer token"}`.
Optional native gRPC check: `brew install grpcurl && grpcurl -plaintext -H "authorization: Bearer $TOKEN" localhost:8080 lessons.v1.LessonsService/ListCourses` (reflection is not registered, so add `-protoset`/`-import-path proto -proto lessons/v1/lessons.proto`). Stop the API afterwards (`kill %1`).

- [ ] **Step 5: CI** — `.github/workflows/api.yml`, FIND (both occurrences, under `push:` and `pull_request:`):

```yaml
    paths:
      - 'apps/api/**'
      - '.github/workflows/api.yml'
```

REPLACE:

```yaml
    paths:
      - 'apps/api/**'
      - 'proto/**'
      - 'buf.yaml'
      - 'buf.gen.yaml'
      - '.github/workflows/api.yml'
```

FIND:

```yaml
      - name: Migrate (goose, embedded)
        run: go run ./cmd/migrate up
```

REPLACE:

```yaml
      - name: Proto codegen is committed and current
        working-directory: .
        run: |
          npx -y @bufbuild/buf@1.73.0 lint
          npx -y @bufbuild/buf@1.73.0 generate
          git diff --exit-code -- apps/api/internal/gen apps/web/gen

      - name: Migrate (goose, embedded)
        run: go run ./cmd/migrate up
```

- [ ] **Step 6: CLAUDE.md** — FIND:

```
- **Media uploads go through the API** (multipart `POST /v1/admin/media`); the type is sniffed from the bytes (png/jpeg/webp/gif ≤ 10 MB, no SVG). Without `R2_*` (dev) files land in `apps/api/tmp/media`; production refuses to start without R2.
```

REPLACE:

```
- **Media uploads go through the API** (multipart `POST /v1/admin/media`); the type is sniffed from the bytes (png/jpeg/webp/gif ≤ 10 MB, no SVG). Without `R2_*` (dev) files land in `apps/api/tmp/media`; production refuses to start without R2.
- **Lessons (self-study tracker) are the one admin API over Connect/gRPC**, not REST: contract in `proto/lessons/v1/lessons.proto`, served at `/lessons.v1.LessonsService/*` beside Gin on the same port, auth via `apphttp.RPCAuth` (`lessons:write`). Generated code is committed; after editing the proto run `cd apps/web && pnpm proto:gen`. h2c is on locally so `grpcurl -plaintext` works; Render only carries Connect over HTTP/1.1. See `docs/adr/0001-connect-protocol-for-lessons-rpc.md`.
```

- [ ] **Step 7: Full test run, lint, commit, PR**

```bash
cd apps/api && TEST_DATABASE_URL=postgres://qdjr:qdjr@localhost:5432/qdjr?sslmode=disable go test -race ./... && golangci-lint run
git add apps/api/cmd/api/main.go .github/workflows/api.yml CLAUDE.md
git commit -m "feat(api): serve lessons.v1.LessonsService over Connect beside Gin with h2c"
git push -u origin feature/dainq/lessons-rpc-api
gh pr create --base main --title "feat(api): lessons tracker served over Connect/gRPC" --body-file - <<'EOF'
## Summary
- `lessons.v1.LessonsService` (8 unary RPCs) defined in `proto/`, generated with buf (Go + TS committed, CI drift check)
- `courses` + `lessons` tables (migration 0010), `lessons:write` for super_admin/editor
- connect-go handler mounted beside Gin on the same port; auth as one Connect interceptor; h2c for local native gRPC
- Render has no native gRPC ingress, so prod is Connect over HTTP/1.1 (ADR 0001)

## Test plan
- [ ] `go test -race ./...` with `TEST_DATABASE_URL` (repo tests run)
- [ ] `curl -X POST /lessons.v1.LessonsService/ListCourses` with a bearer → JSON; without → `unauthenticated`
- [ ] `grpcurl -plaintext` locally → same answer over native gRPC
- [ ] `curl /v1/posts` unchanged
- [ ] After Render deploy: same curl against `https://api.qdjr.dev`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```
Squash-merge after CI is green (one commit on main).

## PR 2 — Admin UI (`feature/dainq/lessons-admin-ui`, after PR 1 is merged)

### Task 8: Connect client, status helpers, navigation

**Files:**
- Modify: `apps/web/package.json`
- Create: `apps/web/lib/rpc.ts`, `apps/web/lib/lesson-status.ts`
- Modify: `apps/web/app/admin/nav.ts`

**Interfaces:**
- Consumes `@/gen/lessons/v1/lessons_pb` (`LessonsService`, `LessonStatus`), `getAccessToken`, `refreshSession`, `signOut` from `@/lib/auth`.
- Produces `lessonsClient` (methods `listCourses`, `getCourse`, `createCourse`, `updateCourse`, `deleteCourse`, `createLesson`, `updateLesson`, `deleteLesson`, each taking a partial request object and returning the response message), `rpcErrorMessage(err: unknown, fallback?: string): string`, `LESSON_STATUSES`, `statusKey(s: LessonStatus): LessonStatusKey`, `statusFromKey(k: string): LessonStatus`, `statusLabel(s: LessonStatus): string`, type `LessonStatusKey = 'todo' | 'in_progress' | 'done'`.

- [ ] **Step 1: Branch and dependencies**

```bash
git fetch origin && git checkout -b feature/dainq/lessons-admin-ui origin/main
cd apps/web && pnpm add @connectrpc/connect@2.2.0 @connectrpc/connect-web@2.2.0 @bufbuild/protobuf@2.16.0
pnpm exec tsc --noEmit
```
Expected: clean (the generated `gen/lessons/v1/lessons_pb.ts` from PR 1 now resolves its `@bufbuild/protobuf` import).

- [ ] **Step 2: Write `lib/rpc.ts`** (full contents)

```ts
// Connect client for the lessons service — the one admin API served over
// Connect/gRPC instead of REST (docs/adr/0001). Fetch-based, so it runs in the
// browser and in the OpenNext worker alike. Auth mirrors lib/api.ts: bearer on
// every call, one refresh-and-retry on Unauthenticated, sign out when the
// refresh fails.
import { Code, ConnectError, createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

import { LessonsService } from '@/gen/lessons/v1/lessons_pb';

import { getAccessToken, refreshSession, signOut } from './auth';

const BASE = process.env.NEXT_PUBLIC_API_URL!;

const auth: Interceptor = (next) => async (req) => {
  const token = await getAccessToken();
  if (token) req.header.set('Authorization', `Bearer ${token}`);
  try {
    return await next(req);
  } catch (err) {
    if (!(err instanceof ConnectError) || err.code !== Code.Unauthenticated) throw err;
    const fresh = await refreshSession();
    if (!fresh) {
      await signOut();
      throw err;
    }
    // Unary only: the request message is a plain object, safe to resend.
    req.header.set('Authorization', `Bearer ${fresh}`);
    return next(req);
  }
};

const transport = createConnectTransport({ baseUrl: BASE, interceptors: [auth] });

export const lessonsClient = createClient(LessonsService, transport);

/** Toast text in the same "CODE: message" shape ApiError produces for REST. */
export function rpcErrorMessage(err: unknown, fallback = 'Request failed'): string {
  if (err instanceof ConnectError) return `${Code[err.code]}: ${err.rawMessage}`;
  if (err instanceof Error) return err.message;
  return fallback;
}
```

- [ ] **Step 3: Write `lib/lesson-status.ts`** (full contents)

```ts
import { LessonStatus } from '@/gen/lessons/v1/lessons_pb';

export type LessonStatusKey = 'todo' | 'in_progress' | 'done';

/** Order is the display order in selects. */
export const LESSON_STATUSES: ReadonlyArray<{
  value: LessonStatus;
  key: LessonStatusKey;
  label: string;
}> = [
  { value: LessonStatus.TODO, key: 'todo', label: 'To do' },
  { value: LessonStatus.IN_PROGRESS, key: 'in_progress', label: 'In progress' },
  { value: LessonStatus.DONE, key: 'done', label: 'Done' },
];

export function statusKey(s: LessonStatus): LessonStatusKey {
  return LESSON_STATUSES.find((x) => x.value === s)?.key ?? 'todo';
}

export function statusFromKey(k: string): LessonStatus {
  return LESSON_STATUSES.find((x) => x.key === k)?.value ?? LessonStatus.TODO;
}

export function statusLabel(s: LessonStatus): string {
  return LESSON_STATUSES.find((x) => x.value === s)?.label ?? 'To do';
}
```

- [ ] **Step 4: Navigation** — `apps/web/app/admin/nav.ts`, FIND:

```ts
import {
  Folder,
  Image as ImageIcon,
```

REPLACE:

```ts
import {
  Folder,
  GraduationCap,
  Image as ImageIcon,
```

FIND:

```ts
  {
    label: 'Site',
    items: [
```

REPLACE:

```ts
  {
    label: 'Learn',
    items: [{ href: '/admin/lessons', label: 'Lessons', icon: GraduationCap }],
  },
  {
    label: 'Site',
    items: [
```

- [ ] **Step 5: Type-check, lint, commit**

```bash
cd apps/web && pnpm exec tsc --noEmit && pnpm lint
git add apps/web/package.json apps/web/pnpm-lock.yaml apps/web/lib/rpc.ts apps/web/lib/lesson-status.ts apps/web/app/admin/nav.ts
git commit -m "feat(web): Connect client for the lessons service and Learn nav group"
```

### Task 9: Courses list screen

**Files:**
- Create: `apps/web/app/admin/lessons/page.tsx`, `apps/web/app/admin/lessons/courses-table.tsx`, `apps/web/app/admin/lessons/course-dialog.tsx`, `apps/web/app/admin/lessons/lesson-chip.tsx`

**Interfaces:**
- Consumes `lessonsClient`, `rpcErrorMessage` (Task 8), `PageHeader` (`@/components/page-header`), shadcn `Table*`, `Dialog*`, `DropdownMenu*`, `Button`, `Input`, `Label`, `Textarea`, `Skeleton`, `timeAgo` (`@/lib/format`), `timestampDate` (`@bufbuild/protobuf/wkt`).
- Produces `CoursesTable`, `CourseDialog({ open, onOpenChange, course? })`, `LessonChip({ status })`.
- Query keys: `['courses', page, PER_PAGE, q]`, invalidated with `['courses']`.

- [ ] **Step 1: `lesson-chip.tsx`** (full contents; same look as `StatusChip`, which is typed to post statuses)

```tsx
import { LessonStatus } from '@/gen/lessons/v1/lessons_pb';
import { statusKey, statusLabel } from '@/lib/lesson-status';
import { cn } from '@/lib/utils';

const DOT: Record<ReturnType<typeof statusKey>, string> = {
  done: 'bg-primary',
  in_progress: 'border border-primary bg-transparent',
  todo: 'border border-muted-foreground bg-transparent',
};

export function LessonChip({ status }: { status: LessonStatus }) {
  const key = statusKey(status);
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-[0.06em]',
        key === 'todo' ? 'text-muted-foreground' : 'text-foreground'
      )}
    >
      <span aria-hidden="true" className={cn('size-1.5 rounded-full', DOT[key])} />
      {statusLabel(status)}
    </span>
  );
}
```

- [ ] **Step 2: `course-dialog.tsx`** (full contents)

```tsx
'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import type { Course } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage } from '@/lib/rpc';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

const schema = z.object({
  title: z.string().trim().min(1, 'Required').max(200),
  url: z.string().trim().url('Must be a URL').or(z.literal('')),
  provider: z.string().trim().max(100),
  notes: z.string().max(10_000),
});

type FormValues = z.infer<typeof schema>;

interface CourseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  course?: Course | null;
}

function defaultValues(c?: Course | null): FormValues {
  return {
    title: c?.title ?? '',
    url: c?.url ?? '',
    provider: c?.provider ?? '',
    notes: c?.notes ?? '',
  };
}

export function CourseDialog({ open, onOpenChange, course }: CourseDialogProps) {
  const queryClient = useQueryClient();
  const isEdit = Boolean(course);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: defaultValues(course) });

  useEffect(() => {
    if (open) reset(defaultValues(course));
  }, [open, course, reset]);

  const done = () => {
    queryClient.invalidateQueries({ queryKey: ['courses'] });
    if (course) queryClient.invalidateQueries({ queryKey: ['course', course.id] });
    onOpenChange(false);
  };

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.createCourse(values),
    onSuccess: done,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Create failed')),
  });

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.updateCourse({ id: course!.id, ...values }),
    onSuccess: done,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Save failed')),
  });

  const onSubmit = handleSubmit((values) => {
    if (isEdit) updateMutation.mutate(values);
    else createMutation.mutate(values);
  });

  const submitting = isSubmitting || createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit course' : 'New course'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-title">Title</Label>
            <Input id="course-title" {...register('title')} placeholder="Course title" />
            {errors.title ? <p className="text-xs text-destructive">{errors.title.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-url">URL</Label>
            <Input id="course-url" {...register('url')} placeholder="https://…" />
            {errors.url ? <p className="text-xs text-destructive">{errors.url.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-provider">Provider</Label>
            <Input id="course-provider" {...register('provider')} placeholder="go.dev, a book, a platform" />
            {errors.provider ? <p className="text-xs text-destructive">{errors.provider.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="course-notes">Notes</Label>
            <Textarea id="course-notes" {...register('notes')} rows={4} placeholder="Markdown supported" className="font-mono text-[13px] leading-6" />
            {errors.notes ? <p className="text-xs text-destructive">{errors.notes.message}</p> : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Saving…' : isEdit ? 'Save' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 3: `courses-table.tsx`** (full contents)

```tsx
'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MoreHorizontal } from 'lucide-react';
import { toast } from 'sonner';

import type { Course } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage } from '@/lib/rpc';
import { timeAgo } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import { CourseDialog } from './course-dialog';

const PER_PAGE = 20;

export function CoursesTable() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [q, setQ] = useState('');
  const [qInput, setQInput] = useState('');
  const [pendingDelete, setPendingDelete] = useState<Course | null>(null);
  const [editTarget, setEditTarget] = useState<Course | null>(null);
  const [createOpen, setCreateOpen] = useState(false);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['courses', page, PER_PAGE, q] as const,
    queryFn: () => lessonsClient.listCourses({ page, perPage: PER_PAGE, q }),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => lessonsClient.deleteCourse({ id }),
    onSuccess: () => {
      setPendingDelete(null);
      queryClient.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (err) => toast.error(rpcErrorMessage(err, 'Delete failed')),
  });

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rows = data?.courses ?? [];

  const onSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    setQ(qInput.trim());
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <form onSubmit={onSearchSubmit} className="flex w-full min-w-0 items-center gap-2 sm:w-auto">
          <Input
            placeholder="Search title…"
            value={qInput}
            onChange={(e) => setQInput(e.target.value)}
            className="min-w-0 flex-1 sm:w-64 sm:flex-none"
          />
          <Button type="submit" variant="outline" size="sm">
            Search
          </Button>
        </form>
        <Button onClick={() => setCreateOpen(true)}>New course</Button>
      </div>

      <div className="overflow-x-auto rounded-[10px] border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Title</TableHead>
              <TableHead>Provider</TableHead>
              <TableHead>Progress</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead className="w-[60px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={`skeleton-${i}`}>
                  <TableCell><Skeleton className="h-4 w-48" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                  <TableCell />
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-destructive">
                  {rpcErrorMessage(error, 'Failed to load courses')}
                </TableCell>
              </TableRow>
            ) : rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  No courses yet.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((course) => (
                <TableRow key={course.id}>
                  <TableCell className="font-medium">
                    <Link href={`/admin/lessons/${course.id}`} className="hover:underline">
                      {course.title}
                    </Link>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{course.provider || '—'}</TableCell>
                  <TableCell className="font-mono text-[11px] uppercase tracking-[0.06em]">
                    {course.doneCount}/{course.lessonCount}
                  </TableCell>
                  <TableCell className="font-mono text-[11px] text-muted-foreground">
                    {course.updatedAt ? timeAgo(timestampDate(course.updatedAt).toISOString()) : '—'}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Actions" />}>
                        <MoreHorizontal className="size-4" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditTarget(course)}>Edit</DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem variant="destructive" onClick={() => setPendingDelete(course)}>
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          {total} course{total === 1 ? '' : 's'}
        </span>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" disabled={page <= 1 || isLoading} onClick={() => setPage((p) => Math.max(1, p - 1))}>
            Previous
          </Button>
          <span>
            Page {page} of {totalPages}
          </span>
          <Button variant="outline" size="sm" disabled={page >= totalPages || isLoading} onClick={() => setPage((p) => p + 1)}>
            Next
          </Button>
        </div>
      </div>

      <CourseDialog open={createOpen} onOpenChange={setCreateOpen} />
      <CourseDialog open={editTarget !== null} onOpenChange={(open) => !open && setEditTarget(null)} course={editTarget} />

      <Dialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete course?</DialogTitle>
            <DialogDescription>
              This deletes &quot;{pendingDelete?.title}&quot; and all its lessons. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)} disabled={deleteMutation.isPending}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={() => pendingDelete && deleteMutation.mutate(pendingDelete.id)} disabled={deleteMutation.isPending}>
              {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
```

- [ ] **Step 4: `page.tsx`** (full contents)

```tsx
import { PageHeader } from '@/components/page-header';

import { CoursesTable } from './courses-table';

export default function LessonsListPage() {
  return (
    <>
      <PageHeader title="Lessons" />
      <CoursesTable />
    </>
  );
}
```

- [ ] **Step 5: Run it against the local API (PR 1 merged, API running), check the screen**

```bash
cd apps/web && pnpm exec tsc --noEmit && pnpm lint && pnpm dev
```
Open `http://localhost:3000/admin/lessons`: "Lessons" header, "New course" creates a row, progress shows `0/0`, Edit/Delete work, no success toast (silent), error toast on a blank title is prevented by the form (zod "Required").

- [ ] **Step 6: Commit**

```bash
git add apps/web/app/admin/lessons
git commit -m "feat(web): courses list screen for the lessons tracker"
```

### Task 10: Course detail with lessons

**Files:**
- Create: `apps/web/app/admin/lessons/[id]/page.tsx`, `apps/web/app/admin/lessons/[id]/course-client.tsx`, `apps/web/app/admin/lessons/lesson-dialog.tsx`

**Interfaces:**
- Consumes `lessonsClient`, `rpcErrorMessage`, `LESSON_STATUSES`, `statusKey`, `statusFromKey`, `LessonChip`, `CourseDialog`, shadcn `Select*`.
- Query key `['course', id]`; mutations invalidate `['course', id]` and `['courses']`.

- [ ] **Step 1: `lesson-dialog.tsx`** (full contents)

```tsx
'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { toast } from 'sonner';

import type { Lesson } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage } from '@/lib/rpc';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

const schema = z.object({
  title: z.string().trim().min(1, 'Required').max(200),
  url: z.string().trim().url('Must be a URL').or(z.literal('')),
  notes: z.string().max(10_000),
});

type FormValues = z.infer<typeof schema>;

interface LessonDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  courseId: string;
  lesson?: Lesson | null;
}

function defaultValues(l?: Lesson | null): FormValues {
  return { title: l?.title ?? '', url: l?.url ?? '', notes: l?.notes ?? '' };
}

export function LessonDialog({ open, onOpenChange, courseId, lesson }: LessonDialogProps) {
  const queryClient = useQueryClient();
  const isEdit = Boolean(lesson);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: defaultValues(lesson) });

  useEffect(() => {
    if (open) reset(defaultValues(lesson));
  }, [open, lesson, reset]);

  const done = () => {
    queryClient.invalidateQueries({ queryKey: ['course', courseId] });
    queryClient.invalidateQueries({ queryKey: ['courses'] });
    onOpenChange(false);
  };

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.createLesson({ courseId, ...values }),
    onSuccess: done,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Create failed')),
  });

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) => lessonsClient.updateLesson({ id: lesson!.id, ...values }),
    onSuccess: done,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Save failed')),
  });

  const onSubmit = handleSubmit((values) => {
    if (isEdit) updateMutation.mutate(values);
    else createMutation.mutate(values);
  });

  const submitting = isSubmitting || createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit lesson' : 'New lesson'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lesson-title">Title</Label>
            <Input id="lesson-title" {...register('title')} placeholder="Lesson title" />
            {errors.title ? <p className="text-xs text-destructive">{errors.title.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lesson-url">URL</Label>
            <Input id="lesson-url" {...register('url')} placeholder="https://…" />
            {errors.url ? <p className="text-xs text-destructive">{errors.url.message}</p> : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lesson-notes">Notes</Label>
            <Textarea id="lesson-notes" {...register('notes')} rows={6} placeholder="Markdown supported" className="font-mono text-[13px] leading-6" />
            {errors.notes ? <p className="text-xs text-destructive">{errors.notes.message}</p> : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Saving…' : isEdit ? 'Save' : 'Add'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 2: `[id]/page.tsx`** (full contents)

```tsx
import { CourseClient } from './course-client';

export default async function CoursePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <CourseClient id={id} />;
}
```

- [ ] **Step 3: `[id]/course-client.tsx`** (full contents)

```tsx
'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MoreHorizontal } from 'lucide-react';
import { toast } from 'sonner';

import type { Lesson } from '@/gen/lessons/v1/lessons_pb';
import { lessonsClient, rpcErrorMessage } from '@/lib/rpc';
import { LESSON_STATUSES, statusFromKey, statusKey } from '@/lib/lesson-status';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import { CourseDialog } from '../course-dialog';
import { LessonChip } from '../lesson-chip';
import { LessonDialog } from '../lesson-dialog';

export function CourseClient({ id }: { id: string }) {
  const queryClient = useQueryClient();
  const [editCourse, setEditCourse] = useState(false);
  const [addLesson, setAddLesson] = useState(false);
  const [editLesson, setEditLesson] = useState<Lesson | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Lesson | null>(null);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['course', id],
    queryFn: () => lessonsClient.getCourse({ id }),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['course', id] });
    queryClient.invalidateQueries({ queryKey: ['courses'] });
  };

  const statusMutation = useMutation({
    mutationFn: ({ lessonId, key }: { lessonId: string; key: string }) =>
      lessonsClient.updateLesson({ id: lessonId, status: statusFromKey(key) }),
    onSuccess: invalidate,
    onError: (err) => toast.error(rpcErrorMessage(err, 'Status change failed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (lessonId: string) => lessonsClient.deleteLesson({ id: lessonId }),
    onSuccess: () => {
      setPendingDelete(null);
      invalidate();
    },
    onError: (err) => toast.error(rpcErrorMessage(err, 'Delete failed')),
  });

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    );
  }
  if (isError) return <p className="text-sm text-destructive">{rpcErrorMessage(error, 'Failed to load course')}</p>;
  if (!data?.course) return null;

  const course = data.course;
  const lessons = data.lessons;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={course.title}
        meta={
          <>
            {course.provider || 'no provider'} · {course.doneCount}/{course.lessonCount} done
            {course.url ? (
              <>
                {' · '}
                <a href={course.url} target="_blank" rel="noreferrer" className="hover:underline">
                  open
                </a>
              </>
            ) : null}
          </>
        }
        actions={
          <>
            <Button variant="outline" onClick={() => setEditCourse(true)}>
              Edit course
            </Button>
            <Button onClick={() => setAddLesson(true)}>New lesson</Button>
          </>
        }
      />

      {course.notes ? (
        <pre className="whitespace-pre-wrap rounded-[10px] border bg-card p-4 font-mono text-[13px] leading-6">{course.notes}</pre>
      ) : null}

      <div className="overflow-x-auto rounded-[10px] border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[48px]">#</TableHead>
              <TableHead>Lesson</TableHead>
              <TableHead className="w-[180px]">Status</TableHead>
              <TableHead>Completed</TableHead>
              <TableHead className="w-[60px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {lessons.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  No lessons yet.
                </TableCell>
              </TableRow>
            ) : (
              lessons.map((lesson) => (
                <TableRow key={lesson.id}>
                  <TableCell className="font-mono text-[11px] text-muted-foreground">{lesson.position}</TableCell>
                  <TableCell className="font-medium">
                    {lesson.url ? (
                      <a href={lesson.url} target="_blank" rel="noreferrer" className="hover:underline">
                        {lesson.title}
                      </a>
                    ) : (
                      lesson.title
                    )}
                    {lesson.notes ? <p className="mt-1 text-xs text-muted-foreground line-clamp-2">{lesson.notes}</p> : null}
                  </TableCell>
                  <TableCell>
                    <Select
                      value={statusKey(lesson.status)}
                      onValueChange={(key) => key && statusMutation.mutate({ lessonId: lesson.id, key })}
                    >
                      <SelectTrigger size="sm" aria-label={`Status of ${lesson.title}`}>
                        <SelectValue>
                          <LessonChip status={lesson.status} />
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        {LESSON_STATUSES.map((s) => (
                          <SelectItem key={s.key} value={s.key}>
                            {s.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell className="font-mono text-[11px] text-muted-foreground">
                    {lesson.completedAt ? timestampDate(lesson.completedAt).toLocaleDateString() : '—'}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Actions" />}>
                        <MoreHorizontal className="size-4" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditLesson(lesson)}>Edit</DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem variant="destructive" onClick={() => setPendingDelete(lesson)}>
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <CourseDialog open={editCourse} onOpenChange={setEditCourse} course={course} />
      <LessonDialog open={addLesson} onOpenChange={setAddLesson} courseId={id} />
      <LessonDialog open={editLesson !== null} onOpenChange={(open) => !open && setEditLesson(null)} courseId={id} lesson={editLesson} />

      <Dialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete lesson?</DialogTitle>
            <DialogDescription>This deletes &quot;{pendingDelete?.title}&quot;. This cannot be undone.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)} disabled={deleteMutation.isPending}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={() => pendingDelete && deleteMutation.mutate(pendingDelete.id)} disabled={deleteMutation.isPending}>
              {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
```

- [ ] **Step 4: Check in the browser**

Open a course from `/admin/lessons`: header shows `provider · 0/0 done`; "New lesson" adds row `#1`; changing its status to Done shows the date in Completed and the header reads `1/1 done`; back to To do clears the date. `pnpm exec tsc --noEmit && pnpm lint && pnpm build` clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/app/admin/lessons
git commit -m "feat(web): course detail with lesson status tracking"
```

### Task 11: End-to-end test and PR

**Files:**
- Create: `apps/web/e2e/lessons.spec.ts`

- [ ] **Step 1: Write the spec** (full contents; the `login` helper is copied, as the other specs do — there is no shared helper)

```ts
import { test, expect, type Page } from '@playwright/test';

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@qdjr.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'ChangeMeNow123!';

function randomSuffix() {
  return Math.random().toString(36).slice(2, 8);
}

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Email').fill(ADMIN_EMAIL);
  await page.getByLabel('Password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/admin(\/|$)/, { timeout: 15_000 });
}

test('lessons: create course, add lesson, mark done, progress updates, delete course', async ({ page }) => {
  const suffix = randomSuffix();
  const courseTitle = `e2e-course-${suffix}`;
  const lessonTitle = `e2e-lesson-${suffix}`;

  await login(page);

  await page.goto('/admin/lessons');
  await expect(page.getByRole('heading', { name: 'Lessons' })).toBeVisible();

  await page.getByRole('button', { name: 'New course' }).click();
  const courseDialog = page.getByRole('dialog');
  await courseDialog.getByLabel('Title').fill(courseTitle);
  await courseDialog.getByLabel('Provider').fill('e2e');
  await courseDialog.getByRole('button', { name: 'Create' }).click();
  await expect(courseDialog).toBeHidden();

  const row = page.getByRole('row', { name: new RegExp(courseTitle) });
  await expect(row).toContainText('0/0');
  await row.getByRole('link', { name: courseTitle }).click();
  await expect(page.getByRole('heading', { name: courseTitle })).toBeVisible();

  await page.getByRole('button', { name: 'New lesson' }).click();
  const lessonDialog = page.getByRole('dialog');
  await lessonDialog.getByLabel('Title').fill(lessonTitle);
  await lessonDialog.getByRole('button', { name: 'Add' }).click();
  await expect(lessonDialog).toBeHidden();
  await expect(page.getByText(lessonTitle)).toBeVisible();
  await expect(page.getByText('0/1 done')).toBeVisible();

  await page.getByRole('combobox', { name: `Status of ${lessonTitle}` }).click();
  await page.getByRole('option', { name: 'Done' }).click();
  await expect(page.getByText('1/1 done')).toBeVisible();

  // Cleanup through the UI so the test leaves no rows behind.
  await page.goto('/admin/lessons');
  const rowAgain = page.getByRole('row', { name: new RegExp(courseTitle) });
  await expect(rowAgain).toContainText('1/1');
  await rowAgain.getByRole('button', { name: 'Actions' }).click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
  await expect(page.getByRole('row', { name: new RegExp(courseTitle) })).toHaveCount(0);
});
```

- [ ] **Step 2: Run it (API on :8080 and `pnpm dev` on :3000 running)**

```bash
cd apps/web && pnpm test:e2e e2e/lessons.spec.ts
```
Expected: 1 passed. If the combobox role differs for the base-ui Select, inspect with `pnpm exec playwright codegen http://localhost:3000` and adjust the two status locators only.

- [ ] **Step 3: Commit and PR**

```bash
git add apps/web/e2e/lessons.spec.ts
git commit -m "test(web): e2e lessons flow"
git push -u origin feature/dainq/lessons-admin-ui
gh pr create --base main --title "feat(web): lessons tracker admin screens over Connect" --body-file - <<'EOF'
## Summary
- `lib/rpc.ts`: connect-web transport with bearer + refresh-once interceptor; `lessonsClient`
- `/admin/lessons` courses list (progress `done/total`), `/admin/lessons/[id]` lessons with per-row status select
- "Learn" nav group; generated TS types from `proto/`
- Playwright `lessons.spec.ts`

## Test plan
- [ ] `pnpm exec tsc --noEmit && pnpm lint && pnpm build`
- [ ] `pnpm test:e2e e2e/lessons.spec.ts` against local API
- [ ] Workers deploy: `/admin/lessons` lists courses from `api.qdjr.dev` (Connect over HTTP/1.1)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

## PR 3 — Suggested curriculum seed (`feature/dainq/lessons-seed`, after PR 1 is merged)

### Task 12: Seed migration from the owner's profile

**Files:**
- Create: `apps/api/migrations/0011_seed_lessons.sql`

**Interfaces:** consumes tables from Task 3 only.

Why these four: the profile (`0009_seed_site_content.sql`) says expert PHP (Yii2/Laravel), MySQL/Redis/Elasticsearch, RabbitMQ; proficient Node/Python, Kafka, K8s. This repo moved the owner to Go + Gin, Next.js 16 on Workers, Neon Postgres, R2, Render, and now Connect/gRPC. The courses fill exactly those gaps; nothing already "expert" is repeated.

- [ ] **Step 1: Verify every URL resolves** (run from repo root; each line must print 200 or 3xx)

```bash
for u in \
  https://protobuf.dev/programming-guides/proto3/ \
  https://protobuf.dev/programming-guides/style/ \
  https://grpc.io/docs/what-is-grpc/core-concepts/ \
  https://connectrpc.com/docs/protocol/ \
  https://connectrpc.com/docs/faq/ \
  https://connectrpc.com/docs/go/getting-started/ \
  https://connectrpc.com/docs/go/interceptors/ \
  https://connectrpc.com/docs/go/errors/ \
  https://connectrpc.com/docs/go/testing/ \
  https://connectrpc.com/docs/go/deployment/ \
  https://connectrpc.com/docs/go/streaming/ \
  https://connectrpc.com/docs/web/using-clients/ \
  https://buf.build/docs/generate/tutorial/ \
  https://github.com/connectrpc/connect-go/releases/tag/v2.0.0 \
  https://go.dev/doc/effective_go \
  https://go.dev/blog/context \
  https://go.dev/blog/pipelines \
  https://pkg.go.dev/golang.org/x/sync/errgroup \
  https://pkg.go.dev/github.com/jackc/pgx/v5 \
  https://pkg.go.dev/testing \
  https://go.dev/blog/pprof \
  https://go.dev/doc/go1.27 \
  https://neon.com/docs/introduction/architecture-overview \
  https://neon.com/docs/connect/connection-pooling \
  https://neon.com/docs/introduction/branching \
  https://www.postgresql.org/docs/current/using-explain.html \
  https://www.postgresql.org/docs/current/indexes.html \
  https://www.postgresql.org/docs/18/release-18.html \
  https://nextjs.org/docs/app/getting-started/server-and-client-components \
  https://nextjs.org/docs/app/getting-started/caching-and-revalidating \
  https://opennext.js.org/cloudflare \
  https://developers.cloudflare.com/workers/platform/limits/ \
  https://tanstack.com/query/latest/docs/framework/react/guides/queries ; do
  printf '%s %s\n' "$(curl -s -o /dev/null -w '%{http_code}' -L "$u")" "$u"; done
```
Any 404: replace that one lesson's URL with the page the site's search returns for the same title before writing the SQL.

- [ ] **Step 2: Branch and write the migration** `apps/api/migrations/0011_seed_lessons.sql` (full contents; fixed ids so it is idempotent and editable in the admin afterwards)

```bash
git fetch origin && git checkout -b feature/dainq/lessons-seed origin/main
```

```sql
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
  ('b0000000-0000-4000-8000-000000000109', 'a0000000-0000-4000-8000-000000000001', 9,  'Testing Connect handlers',            'https://connectrpc.com/docs/go/testing/'),
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
```

- [ ] **Step 3: Apply, check, commit, PR**

```bash
cd apps/api && env $(grep -v '^#' .env | xargs) go run ./cmd/migrate up && go test ./migrations
psql postgres://qdjr:qdjr@localhost:5432/qdjr -c "select c.title, count(l.id) from public.courses c left join public.lessons l on l.course_id=c.id group by c.id order by c.title"
```
Expected: 4 rows with 14 / 8 / 6 / 5 lessons; a second `migrate up` is a no-op. In the admin, `/admin/lessons` lists the four courses at `0/N`.

```bash
git add apps/api/migrations/0011_seed_lessons.sql
git commit -m "feat(api): seed suggested self-study courses from the owner's profile"
git push -u origin feature/dainq/lessons-seed
gh pr create --base main --title "feat(api): seed suggested lessons curriculum" --body-file - <<'EOF'
## Summary
- Migration 0011: four courses (Connect/gRPC, Go, Postgres on Neon, Next.js on Workers) with 33 lessons, fixed ids, idempotent
- Topics chosen from the profile's skills vs. this repo's stack

## Test plan
- [ ] `go run ./cmd/migrate up` twice → second run no-op; `go test ./migrations`
- [ ] `/admin/lessons` shows 4 courses after Render applies the migration

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

---

## Verification (end to end)

1. API: `go test -race ./...` with `TEST_DATABASE_URL`; `golangci-lint run`; CI drift step green.
2. Local Connect: `curl -X POST localhost:8080/lessons.v1.LessonsService/ListCourses` with bearer → JSON; without → `unauthenticated`; `curl localhost:8080/v1/posts` unchanged.
3. Local native gRPC: `grpcurl -plaintext -import-path proto -proto lessons/v1/lessons.proto -H "authorization: Bearer $TOKEN" localhost:8080 lessons.v1.LessonsService/ListCourses` → same payload.
4. Admin: `pnpm exec tsc --noEmit && pnpm lint && pnpm build`; `pnpm test:e2e e2e/lessons.spec.ts`.
5. Production: after Render deploys PR 1, repeat step 2 against `https://api.qdjr.dev` (HTTP/1.1 edge, Connect still answers); after PR 2 deploys to Workers, `/admin/lessons` works from the live admin; after PR 3, four seeded courses appear.

## Self-review notes

- Spec coverage: tracker (Tasks 3, 4, 9, 10), Connect transport (2, 5, 6, 7), learning via local native gRPC (6 test, 7 smoke), codegen committed + drift (2, 7), permission (3), suggestions from profile (12), docs/map/ADR (1, 7). Public site untouched by design.
- Review Focus → tests: 1 → Task 6 `TestRPC_CreateCourse_BlankTitle`/`LongTitle`; 2 → Task 4 `TestRepo_CreateLesson_UnknownCourse_NotFound`; 3 → Task 4 `TestRepo_UpdateLesson_DoneStampsCompletedAt`; 4 → Task 5 `TestConnectCORS_*`; 5 → Task 5 `TestRPCAuth_ExpiredToken_Unauthenticated` and Task 6 `TestRPC_ExpiredToken_Unauthenticated`.
- Type consistency: `AdminWriter` method set identical in Task 4 (interface), Task 4 repo, Task 6 stub; `UpdateLessonInput.Status *Status`, `Position *int` used the same way in handler and repo; TS `statusKey`/`statusFromKey` names match between Task 8 and Task 10.
- Known judgement calls: `ConnectCORS(nil)` in tests disables CORS headers (no Origin in Go clients); `Course.created_by` is NULL when the context carries no user (only possible in tests, since RPCAuth always sets it); position reordering is exposed in the RPC (`position`) but the UI does not offer drag-reorder — YAGNI until asked.
- After plan approval, also record in memory: this repo wants execution-ready plans (writing-plans format), same preference as the other repos in `prefs/plans-must-be-execution-ready.md`.

