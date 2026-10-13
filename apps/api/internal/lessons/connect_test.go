package lessons

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/connect/v2/connectproto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

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
	mu             sync.Mutex // WatchCourse polls from the server goroutine
	panicOnList    bool
	courses        map[uuid.UUID]Course
	lessons        map[uuid.UUID]Lesson
	listFilt       ListFilter
	createCourseIn CreateCourseInput
	updateLessonIn UpdateLessonInput
	reorderIDs     []uuid.UUID
	reorderErr     error
	updateCourseIn UpdateCourseInput
	failWith       error // when set, UpdateLesson returns it (exercises mapErr)
}

func newStubRepo() *stubRepo {
	return &stubRepo{courses: map[uuid.UUID]Course{}, lessons: map[uuid.UUID]Lesson{}}
}

func (s *stubRepo) ListCourses(_ context.Context, f ListFilter) ([]Course, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.panicOnList {
		panic("boom")
	}
	s.listFilt = f
	out := make([]Course, 0, len(s.courses))
	for _, c := range s.courses {
		out = append(out, c)
	}
	return out, len(out), nil
}

func (s *stubRepo) GetCourse(_ context.Context, id uuid.UUID) (Course, []Lesson, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	sort.Slice(ls, func(i, j int) bool { return ls[i].Position < ls[j].Position })
	return c, ls, nil
}

func (s *stubRepo) CreateCourse(_ context.Context, in CreateCourseInput) (Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCourseIn = in
	now := time.Now().UTC()
	c := Course{ID: uuid.New(), Title: in.Title, URL: in.URL, Provider: in.Provider, Notes: in.Notes, CreatedAt: now, UpdatedAt: now}
	s.courses[c.ID] = c
	return c, nil
}

func (s *stubRepo) UpdateCourse(_ context.Context, id uuid.UUID, in UpdateCourseInput) (Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateCourseIn = in
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.courses[id]; !ok {
		return ErrNotFound
	}
	delete(s.courses, id)
	return nil
}

func (s *stubRepo) CreateLesson(_ context.Context, in CreateLessonInput) (Lesson, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.courses[in.CourseID]; !ok {
		return Lesson{}, ErrNotFound
	}
	now := time.Now().UTC()
	l := Lesson{ID: uuid.New(), CourseID: in.CourseID, Title: in.Title, URL: in.URL, Notes: in.Notes, Position: len(s.lessons) + 1, Status: StatusTodo, CreatedAt: now, UpdatedAt: now}
	s.lessons[l.ID] = l
	return l, nil
}

func (s *stubRepo) UpdateLesson(_ context.Context, id uuid.UUID, in UpdateLessonInput) (Lesson, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateLessonIn = in
	if s.failWith != nil {
		return Lesson{}, s.failWith
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return newEnvWith(t, nil, perms...)
}

// newEnvWith is newEnv with extra interceptors mounted before RPCAuth.
func newEnvWith(t *testing.T, extra []connect.ServerInterceptor, perms ...string) *env {
	t.Helper()
	repo := newStubRepo()
	res := &resolverStub{perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(testSecret))
	mux := http.NewServeMux()
	server := NewServer(repo, append(slices.Clone(extra), apphttp.RPCAuth(v, res, "lessons:write"))...)
	Mount(apphttp.WrapMux(mux, apphttp.ConnectCORS(nil)), server)

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

// bearer stamps every call, unary or streaming: a v2 client interceptor wraps
// all RPC types, and the client attaches a CallInfo before interceptors run.
func bearer(tok string) connect.ClientInterceptor {
	return func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			if info, ok := connect.CallInfoForClientContext(ctx); ok {
				info.RequestHeader().Set("Authorization", "Bearer "+tok)
			}
			return next(ctx, spec)
		}
	}
}

// newClient builds a lessons client over HTTP; tok "" sends no bearer.
func newClient(hc connecthttp.HTTPClient, baseURL, tok string, opts ...connecthttp.Option) lessonsv1connect.LessonsServiceClient {
	var interceptors []connect.ClientInterceptor
	if tok != "" {
		interceptors = append(interceptors, bearer(tok))
	}
	return lessonsv1connect.NewLessonsServiceClient(connect.NewClient(connecthttp.NewTransport(hc, baseURL, opts...), interceptors...))
}

func (e *env) client(t *testing.T, uid uuid.UUID, opts ...connecthttp.Option) lessonsv1connect.LessonsServiceClient {
	t.Helper()
	return newClient(http.DefaultClient, e.srv.URL, token(t, uid, time.Now().Add(time.Hour)), opts...)
}

func TestRPC_NoToken_Unauthenticated(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := newClient(http.DefaultClient, e.srv.URL, "")
	_, err := c.ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPC_ExpiredToken_Unauthenticated(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := newClient(http.DefaultClient, e.srv.URL, token(t, uuid.New(), time.Now().Add(-time.Minute)))
	_, err := c.ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPC_MissingPermission_PermissionDenied(t *testing.T) {
	e := newEnv(t) // author-like: no lessons:write
	_, err := e.client(t, uuid.New()).ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPC_ListCourses_PassesFilter(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{Page: 2, PerPage: 5, Q: "go"})
	require.NoError(t, err)
	assert.Equal(t, ListFilter{Page: 2, PerPage: 5, Q: "go"}, e.repo.listFilt)
}

func TestRPC_CreateCourse_OK_RecordsCreator(t *testing.T) {
	e := newEnv(t, "lessons:write")
	uid := uuid.New()
	res, err := e.client(t, uid).CreateCourse(context.Background(), &lessonsv1.CreateCourseRequest{Title: "  Effective Go  ", Url: "https://go.dev/doc/effective_go", Provider: "go.dev"})
	require.NoError(t, err)
	assert.Equal(t, "Effective Go", res.GetCourse().GetTitle())
	assert.Equal(t, uid, e.repo.createCourseIn.CreatedBy)
	assert.NotNil(t, res.GetCourse().GetCreatedAt())
}

func TestRPC_CreateCourse_BlankTitle_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(), &lessonsv1.CreateCourseRequest{Title: "   "})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_CreateCourse_LongTitle_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	// 201 multibyte runes: byte length would be 402, rune count is what matters.
	long := strings.Repeat("é", 201)
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(), &lessonsv1.CreateCourseRequest{Title: long})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_GetCourse_BadID_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).GetCourse(context.Background(), &lessonsv1.GetCourseRequest{Id: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_GetCourse_Unknown_NotFound(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).GetCourse(context.Background(), &lessonsv1.GetCourseRequest{Id: uuid.NewString()})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRPC_UpdateLesson_StatusDone(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	course, err := c.CreateCourse(context.Background(), &lessonsv1.CreateCourseRequest{Title: "c"})
	require.NoError(t, err)
	lesson, err := c.CreateLesson(context.Background(), &lessonsv1.CreateLessonRequest{CourseId: course.GetCourse().GetId(), Title: "l"})
	require.NoError(t, err)

	done := lessonsv1.LessonStatus_LESSON_STATUS_DONE
	res, err := c.UpdateLesson(context.Background(), &lessonsv1.UpdateLessonRequest{Id: lesson.GetLesson().GetId(), Status: &done})
	require.NoError(t, err)
	assert.Equal(t, lessonsv1.LessonStatus_LESSON_STATUS_DONE, res.GetLesson().GetStatus())
	assert.NotNil(t, res.GetLesson().GetCompletedAt())
	require.NotNil(t, e.repo.updateLessonIn.Status)
	assert.Equal(t, StatusDone, *e.repo.updateLessonIn.Status)
}

func TestRPC_UpdateLesson_UnspecifiedStatus_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	unspecified := lessonsv1.LessonStatus_LESSON_STATUS_UNSPECIFIED
	_, err := e.client(t, uuid.New()).UpdateLesson(context.Background(), &lessonsv1.UpdateLessonRequest{Id: uuid.NewString(), Status: &unspecified})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRPC_DeleteCourse_Unknown_NotFound(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).DeleteCourse(context.Background(), &lessonsv1.DeleteCourseRequest{Id: uuid.NewString()})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestRPC_NativeGRPC_OverH2C proves the same handler answers real gRPC when
// the client speaks prior-knowledge HTTP/2 (what grpcurl -plaintext does).
func TestRPC_NativeGRPC_OverH2C(t *testing.T) {
	e := newEnv(t, "lessons:write")
	var cp http.Protocols
	cp.SetUnencryptedHTTP2(true) // no HTTP/1 → prior-knowledge h2c
	h2c := &http.Client{Transport: &http.Transport{Protocols: &cp}}
	c := newClient(h2c, e.srv.URL, token(t, uuid.New(), time.Now().Add(time.Hour)), connecthttp.WithGRPC())
	res, err := c.ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(0), res.GetTotal())
}

// TestRPC_OversizedBody_ResourceExhausted: the body is read before the auth
// interceptor runs, so the cap is what stops an anonymous caller from making
// the free instance buffer an arbitrary upload.
func TestRPC_OversizedBody_ResourceExhausted(t *testing.T) {
	e := newEnv(t, "lessons:write")
	huge := strings.Repeat("x", maxRequestBytes+1)
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(), &lessonsv1.CreateCourseRequest{Title: "big", Notes: huge})
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
}

// TestRPC_HandlerPanic_Internal: a panic must surface as CodeInternal, not a
// dropped connection (Gin's Recovery does not wrap the Connect mount).
func TestRPC_HandlerPanic_Internal(t *testing.T) {
	e := newEnv(t, "lessons:write")
	e.repo.panicOnList = true
	_, err := e.client(t, uuid.New()).ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{})
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// TestRPC_RPCLog_LogsProcedureAndCode: one log line per call, including
// calls RPCAuth rejects, with the procedure, the connect code and a duration.
func TestRPC_RPCLog_LogsProcedureAndCode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	e := newEnvWith(t, []connect.ServerInterceptor{apphttp.RPCLog(logger)}, "lessons:write")

	anon := newClient(http.DefaultClient, e.srv.URL, "")
	_, err := anon.ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{})
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	denied := buf.String()
	assert.Contains(t, denied, `"procedure":"/lessons.v1.LessonsService/ListCourses"`)
	assert.Contains(t, denied, `"code":"unauthenticated"`)
	assert.Contains(t, denied, `"duration_ms"`)

	buf.Reset()
	_, err = e.client(t, uuid.New()).ListCourses(context.Background(), &lessonsv1.ListCoursesRequest{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"code":"ok"`)
}

// TestRPC_CreateCourse_LongTitle_FieldViolation: the InvalidArgument error
// carries a google.rpc.BadRequest detail naming the field, decoded by the
// real client over the wire, so a form can show it next to the input.
func TestRPC_CreateCourse_LongTitle_FieldViolation(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(),
		&lessonsv1.CreateCourseRequest{Title: strings.Repeat("é", 201)})
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	var cerr *connect.Error
	require.ErrorAs(t, err, &cerr)
	require.Len(t, cerr.Details(), 1)
	v, derr := connectproto.UnmarshalErrorDetail(cerr.Details()[0])
	require.NoError(t, derr)
	br, ok := v.(*errdetails.BadRequest)
	require.True(t, ok, "detail is %T", v)
	require.Len(t, br.GetFieldViolations(), 1)
	assert.Equal(t, "title", br.GetFieldViolations()[0].GetField())
	assert.Contains(t, br.GetFieldViolations()[0].GetDescription(), "200")
}

func (s *stubRepo) ReorderLessons(_ context.Context, courseID uuid.UUID, ids []uuid.UUID) ([]Lesson, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reorderIDs = ids
	if s.reorderErr != nil {
		return nil, s.reorderErr
	}
	if _, ok := s.courses[courseID]; !ok {
		return nil, ErrNotFound
	}
	out := make([]Lesson, 0, len(ids))
	for i, id := range ids {
		l := s.lessons[id]
		l.Position = i + 1
		out = append(out, l)
	}
	return out, nil
}

func fieldOf(t *testing.T, err error) string {
	t.Helper()
	var cerr *connect.Error
	require.ErrorAs(t, err, &cerr)
	require.NotEmpty(t, cerr.Details())
	v, derr := connectproto.UnmarshalErrorDetail(cerr.Details()[0])
	require.NoError(t, derr)
	br, ok := v.(*errdetails.BadRequest)
	require.True(t, ok)
	return br.GetFieldViolations()[0].GetField()
}

func TestRPC_ReorderLessons_OK_PassesOrder(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	course, err := c.CreateCourse(context.Background(), &lessonsv1.CreateCourseRequest{Title: "c"})
	require.NoError(t, err)
	cid := course.GetCourse().GetId()
	a, err := c.CreateLesson(context.Background(), &lessonsv1.CreateLessonRequest{CourseId: cid, Title: "a"})
	require.NoError(t, err)
	b, err := c.CreateLesson(context.Background(), &lessonsv1.CreateLessonRequest{CourseId: cid, Title: "b"})
	require.NoError(t, err)

	res, err := c.ReorderLessons(context.Background(), &lessonsv1.ReorderLessonsRequest{
		CourseId: cid, LessonIds: []string{b.GetLesson().GetId(), a.GetLesson().GetId()},
	})
	require.NoError(t, err)
	require.Len(t, res.GetLessons(), 2)
	assert.Equal(t, "b", res.GetLessons()[0].GetTitle())
	assert.Equal(t, int32(1), res.GetLessons()[0].GetPosition())
	assert.Len(t, e.repo.reorderIDs, 2)
}

func TestRPC_ReorderLessons_Duplicate_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	id := uuid.NewString()
	_, err := e.client(t, uuid.New()).ReorderLessons(context.Background(), &lessonsv1.ReorderLessonsRequest{
		CourseId: uuid.NewString(), LessonIds: []string{id, id},
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Equal(t, "lesson_ids", fieldOf(t, err))
	assert.Nil(t, e.repo.reorderIDs, "repo must not be called")
}

func TestRPC_ReorderLessons_BadLessonID_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).ReorderLessons(context.Background(), &lessonsv1.ReorderLessonsRequest{
		CourseId: uuid.NewString(), LessonIds: []string{"nope"},
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Equal(t, "lesson_ids", fieldOf(t, err))
}

func TestRPC_ReorderLessons_Mismatch_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	e.repo.reorderErr = ErrLessonSetMismatch
	_, err := e.client(t, uuid.New()).ReorderLessons(context.Background(), &lessonsv1.ReorderLessonsRequest{
		CourseId: uuid.NewString(), LessonIds: []string{uuid.NewString()},
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Equal(t, "lesson_ids", fieldOf(t, err))
}

// syncBuf is a log sink the server goroutine and the test can share.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func fastWatch(t *testing.T) {
	t.Helper()
	prevInterval, prevKeepalive := watchInterval, watchKeepalive
	watchInterval = 20 * time.Millisecond
	t.Cleanup(func() { watchInterval, watchKeepalive = prevInterval, prevKeepalive })
}

// TestRPC_WatchCourse_NoToken_Unauthenticated pins that auth (and logging)
// cover streaming calls, not only unary ones.
func TestRPC_WatchCourse_NoToken_Unauthenticated(t *testing.T) {
	var buf syncBuf
	e := newEnvWith(t, []connect.ServerInterceptor{apphttp.RPCLog(slog.New(slog.NewJSONHandler(&buf, nil)))}, "lessons:write")
	anon := newClient(http.DefaultClient, e.srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := anon.WatchCourse(ctx, &lessonsv1.WatchCourseRequest{Id: uuid.NewString()})
	if err == nil { // the error may surface on open or on the first Receive
		_, err = stream.Receive()
	}
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	assert.Contains(t, buf.String(), `"procedure":"/lessons.v1.LessonsService/WatchCourse"`)
	assert.Contains(t, buf.String(), `"code":"unauthenticated"`)
}

func TestRPC_WatchCourse_UnknownCourse_NotFound(t *testing.T) {
	fastWatch(t)
	e := newEnv(t, "lessons:write")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := e.client(t, uuid.New()).WatchCourse(ctx, &lessonsv1.WatchCourseRequest{Id: uuid.NewString()})
	if err == nil {
		_, err = stream.Receive()
	}
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestRPC_WatchCourse_SnapshotThenChange_GRPC streams over native gRPC on
// h2c (what the cmd/watch client does): a first snapshot, nothing while
// unchanged, then a new snapshot after a lesson is added.
func TestRPC_WatchCourse_SnapshotThenChange_GRPC(t *testing.T) {
	fastWatch(t)
	e := newEnv(t, "lessons:write")
	uid := uuid.New()
	c := e.client(t, uid)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	course, err := c.CreateCourse(ctx, &lessonsv1.CreateCourseRequest{Title: "watched"})
	require.NoError(t, err)
	cid := course.GetCourse().GetId()

	var cp http.Protocols
	cp.SetUnencryptedHTTP2(true)
	h2c := &http.Client{Transport: &http.Transport{Protocols: &cp}}
	g := newClient(h2c, e.srv.URL, token(t, uid, time.Now().Add(time.Hour)), connecthttp.WithGRPC())
	stream, err := g.WatchCourse(ctx, &lessonsv1.WatchCourseRequest{Id: cid})
	require.NoError(t, err)

	first, err := stream.Receive()
	require.NoError(t, err, "first snapshot")
	assert.Equal(t, "watched", first.GetCourse().GetTitle())
	assert.Empty(t, first.GetLessons())

	_, err = c.CreateLesson(ctx, &lessonsv1.CreateLessonRequest{CourseId: cid, Title: "new"})
	require.NoError(t, err)
	// An unchanged poll sends nothing, so the next message is the change.
	changed, err := stream.Receive()
	require.NoError(t, err, "change")
	require.Len(t, changed.GetLessons(), 1)
	assert.Equal(t, "new", changed.GetLessons()[0].GetTitle())
	require.NoError(t, stream.Close())
}

// TestRPC_WatchCourse_KeepaliveResendsWhenIdle: with nothing changing, the
// stream still sends the current snapshot every watchKeepalive, because the
// production edge resets an HTTP/2 stream after ~60s without data.
func TestRPC_WatchCourse_KeepaliveResendsWhenIdle(t *testing.T) {
	fastWatch(t)
	watchKeepalive = 100 * time.Millisecond
	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	course, err := c.CreateCourse(ctx, &lessonsv1.CreateCourseRequest{Title: "idle"})
	require.NoError(t, err)

	stream, err := c.WatchCourse(ctx, &lessonsv1.WatchCourseRequest{Id: course.GetCourse().GetId()})
	require.NoError(t, err)
	_, err = stream.Receive()
	require.NoError(t, err, "first snapshot")
	first := time.Now()
	again, err := stream.Receive()
	require.NoError(t, err, "keepalive")
	assert.GreaterOrEqual(t, time.Since(first), 80*time.Millisecond, "keepalive came too early")
	assert.Equal(t, "idle", again.GetCourse().GetTitle())
	require.NoError(t, stream.Close())
}

func TestCleanTitle_Table(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // "" means rejected
	}{
		{"empty", "", ""},
		{"whitespace only", " \t\n ", ""},
		{"trims surrounding space", "  Effective Go  ", "Effective Go"},
		{"exactly 200 runes", strings.Repeat("a", 200), strings.Repeat("a", 200)},
		{"201 runes", strings.Repeat("a", 201), ""},
		{"200 two-byte runes (400 bytes)", strings.Repeat("é", 200), strings.Repeat("é", 200)},
		{"200 emoji (800 bytes)", strings.Repeat("🦀", 200), strings.Repeat("🦀", 200)},
		{"201 emoji", strings.Repeat("🦀", 201), ""},
		{"200 runes plus padding", "  " + strings.Repeat("a", 200) + "  ", strings.Repeat("a", 200)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cleanTitle(tc.in)
			if tc.want == "" {
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseID_Table(t *testing.T) {
	const canonical = "a0000000-0000-4000-8000-000000000001"
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"canonical", canonical, true},
		{"uppercase", strings.ToUpper(canonical), true},
		{"braces", "{" + canonical + "}", true},
		{"urn prefix", "urn:uuid:" + canonical, true},
		{"empty", "", false},
		{"not a uuid", "nope", false},
		{"trailing space", canonical + " ", false},
		{"one char short", canonical[:35], false},
	}
	want := uuid.MustParse(canonical)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseID(tc.in)
			if !tc.ok {
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, want, got, "every accepted form normalises to the same id")
		})
	}
}

// TestRPC_UpdateLesson_Branches walks every branch of the UpdateLesson handler.
func TestRPC_UpdateLesson_Branches(t *testing.T) {
	str := func(s string) *string { return &s }
	i32 := func(n int32) *int32 { return &n }
	st := func(s lessonsv1.LessonStatus) *lessonsv1.LessonStatus { return &s }

	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	ctx := context.Background()
	course, err := c.CreateCourse(ctx, &lessonsv1.CreateCourseRequest{Title: "c"})
	require.NoError(t, err)
	lesson, err := c.CreateLesson(ctx, &lessonsv1.CreateLessonRequest{CourseId: course.GetCourse().GetId(), Title: "l"})
	require.NoError(t, err)
	id := lesson.GetLesson().GetId()

	rejected := []struct {
		name  string
		req   *lessonsv1.UpdateLessonRequest
		code  connect.Code
		field string // "" when no field violation is expected
	}{
		{"bad id", &lessonsv1.UpdateLessonRequest{Id: "nope"}, connect.CodeInvalidArgument, ""},
		{"blank title", &lessonsv1.UpdateLessonRequest{Id: id, Title: str("   ")}, connect.CodeInvalidArgument, "title"},
		{"unspecified status", &lessonsv1.UpdateLessonRequest{Id: id, Status: st(lessonsv1.LessonStatus_LESSON_STATUS_UNSPECIFIED)}, connect.CodeInvalidArgument, "status"},
		{"unknown status value", &lessonsv1.UpdateLessonRequest{Id: id, Status: st(lessonsv1.LessonStatus(99))}, connect.CodeInvalidArgument, "status"},
		{"negative position", &lessonsv1.UpdateLessonRequest{Id: id, Position: i32(-1)}, connect.CodeInvalidArgument, "position"},
		{"unknown lesson", &lessonsv1.UpdateLessonRequest{Id: uuid.NewString(), Title: str("x")}, connect.CodeNotFound, ""},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.UpdateLesson(ctx, tc.req)
			require.Equal(t, tc.code, connect.CodeOf(err), err)
			if tc.field != "" {
				assert.Equal(t, tc.field, fieldOf(t, err))
			}
		})
	}

	t.Run("every optional field passes through, cleaned", func(t *testing.T) {
		_, err := c.UpdateLesson(ctx, &lessonsv1.UpdateLessonRequest{
			Id: id, Title: str("  Renamed  "), Url: str("  https://go.dev  "), Notes: str("  kept as is  "),
			Status: st(lessonsv1.LessonStatus_LESSON_STATUS_IN_PROGRESS), Position: i32(0),
		})
		require.NoError(t, err)
		in := e.repo.updateLessonIn
		require.NotNil(t, in.Title)
		assert.Equal(t, "Renamed", *in.Title)
		assert.Equal(t, "https://go.dev", *in.URL)
		assert.Equal(t, "  kept as is  ", *in.Notes, "notes are markdown, not trimmed")
		assert.Equal(t, StatusInProgress, *in.Status)
		assert.Equal(t, 0, *in.Position, "0 is a valid position")
	})

	t.Run("unset fields stay nil", func(t *testing.T) {
		_, err := c.UpdateLesson(ctx, &lessonsv1.UpdateLessonRequest{Id: id})
		require.NoError(t, err)
		assert.Equal(t, UpdateLessonInput{}, e.repo.updateLessonIn)
	})

	t.Run("repository failure is Internal", func(t *testing.T) {
		e.repo.mu.Lock()
		e.repo.failWith = errors.New("db down")
		e.repo.mu.Unlock()
		t.Cleanup(func() { e.repo.mu.Lock(); e.repo.failWith = nil; e.repo.mu.Unlock() })
		_, err := c.UpdateLesson(ctx, &lessonsv1.UpdateLessonRequest{Id: id})
		assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		var cerr *connect.Error
		require.ErrorAs(t, err, &cerr)
		assert.NotContains(t, cerr.Message(), "db down", "the database error must not reach the client")
	})
}

func TestRPC_UpdateCourse_Branches(t *testing.T) {
	str := func(s string) *string { return &s }
	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	ctx := context.Background()
	course, err := c.CreateCourse(ctx, &lessonsv1.CreateCourseRequest{Title: "c"})
	require.NoError(t, err)
	id := course.GetCourse().GetId()

	_, err = c.UpdateCourse(ctx, &lessonsv1.UpdateCourseRequest{Id: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad id")
	_, err = c.UpdateCourse(ctx, &lessonsv1.UpdateCourseRequest{Id: id, Title: str(" ")})
	assert.Equal(t, "title", fieldOf(t, err), "blank title")
	_, err = c.UpdateCourse(ctx, &lessonsv1.UpdateCourseRequest{Id: uuid.NewString(), Provider: str("x")})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "unknown course")

	res, err := c.UpdateCourse(ctx, &lessonsv1.UpdateCourseRequest{Id: id, Provider: str("  go.dev  "), Url: str(" https://go.dev ")})
	require.NoError(t, err)
	assert.Equal(t, "go.dev", res.GetCourse().GetProvider())
	assert.Nil(t, e.repo.updateCourseIn.Title, "title untouched when unset")
	assert.Equal(t, "https://go.dev", *e.repo.updateCourseIn.URL)
}

func TestRPC_DeleteLesson_Branches(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	ctx := context.Background()
	course, err := c.CreateCourse(ctx, &lessonsv1.CreateCourseRequest{Title: "c"})
	require.NoError(t, err)
	lesson, err := c.CreateLesson(ctx, &lessonsv1.CreateLessonRequest{CourseId: course.GetCourse().GetId(), Title: "l"})
	require.NoError(t, err)

	_, err = c.DeleteLesson(ctx, &lessonsv1.DeleteLessonRequest{Id: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = c.DeleteLesson(ctx, &lessonsv1.DeleteLessonRequest{Id: lesson.GetLesson().GetId()})
	require.NoError(t, err)
	_, err = c.DeleteLesson(ctx, &lessonsv1.DeleteLessonRequest{Id: lesson.GetLesson().GetId()})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "second delete")
}

// TestRPC_InternalError_CauseLoggedNotSent: v2 sends only the fixed message;
// the cause (here the repository error) reaches the server log via RPCLog.
func TestRPC_InternalError_CauseLoggedNotSent(t *testing.T) {
	var buf syncBuf
	e := newEnvWith(t, []connect.ServerInterceptor{apphttp.RPCLog(slog.New(slog.NewJSONHandler(&buf, nil)))}, "lessons:write")
	c := e.client(t, uuid.New())
	e.repo.mu.Lock()
	e.repo.failWith = errors.New("pg: connection refused at 10.0.0.7")
	e.repo.mu.Unlock()

	_, err := c.UpdateLesson(context.Background(), &lessonsv1.UpdateLessonRequest{Id: uuid.NewString()})
	var cerr *connect.Error
	require.ErrorAs(t, err, &cerr)
	assert.Equal(t, connect.CodeInternal, cerr.Code())
	assert.NotContains(t, cerr.Message(), "10.0.0.7")
	assert.Contains(t, buf.String(), `"cause":"pg: connection refused at 10.0.0.7"`)
}

// TestRPC_AuthBeforeBody: a v2 interceptor runs before the body is read, so a
// token-less oversized call is refused for the token, not the size.
func TestRPC_AuthBeforeBody(t *testing.T) {
	e := newEnv(t, "lessons:write")
	huge := strings.Repeat("x", maxRequestBytes+1)
	_, err := newClient(http.DefaultClient, e.srv.URL, "").CreateCourse(context.Background(), &lessonsv1.CreateCourseRequest{Title: "big", Notes: huge})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
