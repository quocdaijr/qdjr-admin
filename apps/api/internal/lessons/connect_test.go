package lessons

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
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
func newEnvWith(t *testing.T, extra []connect.Interceptor, perms ...string) *env {
	t.Helper()
	repo := newStubRepo()
	res := &resolverStub{perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(testSecret))
	mux := http.NewServeMux()
	path, h := NewConnectHandler(repo, append(extra, apphttp.RPCAuth(v, res, "lessons:write"))...)
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
	return connect.WithInterceptors(bearerInterceptor{value: "Bearer " + tok})
}

// bearerInterceptor stamps unary and streaming calls alike; a
// UnaryInterceptorFunc would silently skip WatchCourse.
type bearerInterceptor struct{ value string }

func (b bearerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", b.value)
		return next(ctx, req)
	}
}

func (b bearerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", b.value)
		return conn
	}
}

func (b bearerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
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
	// 201 multibyte runes: byte length would be 402, rune count is what matters.
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

// TestRPC_OversizedBody_ResourceExhausted: the body is read before the auth
// interceptor runs, so the cap is what stops an anonymous caller from making
// the free instance buffer an arbitrary upload.
func TestRPC_OversizedBody_ResourceExhausted(t *testing.T) {
	e := newEnv(t, "lessons:write")
	huge := strings.Repeat("x", maxRequestBytes+1)
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(), connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: "big", Notes: huge}))
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
}

// TestRPC_HandlerPanic_Internal: a panic must surface as CodeInternal, not a
// dropped connection (Gin's Recovery does not wrap the Connect mount).
func TestRPC_HandlerPanic_Internal(t *testing.T) {
	e := newEnv(t, "lessons:write")
	e.repo.panicOnList = true
	_, err := e.client(t, uuid.New()).ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{}))
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// TestRPC_RPCLog_LogsProcedureAndCode: one log line per call, including
// calls RPCAuth rejects, with the procedure, the connect code and a duration.
func TestRPC_RPCLog_LogsProcedureAndCode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	e := newEnvWith(t, []connect.Interceptor{apphttp.RPCLog(logger)}, "lessons:write")

	anon := lessonsv1connect.NewLessonsServiceClient(http.DefaultClient, e.srv.URL)
	_, err := anon.ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	denied := buf.String()
	assert.Contains(t, denied, `"procedure":"/lessons.v1.LessonsService/ListCourses"`)
	assert.Contains(t, denied, `"code":"unauthenticated"`)
	assert.Contains(t, denied, `"duration_ms"`)

	buf.Reset()
	_, err = e.client(t, uuid.New()).ListCourses(context.Background(), connect.NewRequest(&lessonsv1.ListCoursesRequest{}))
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"code":"ok"`)
}

// TestRPC_CreateCourse_LongTitle_FieldViolation: the InvalidArgument error
// carries a google.rpc.BadRequest detail naming the field, decoded by the
// real client over the wire, so a form can show it next to the input.
func TestRPC_CreateCourse_LongTitle_FieldViolation(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).CreateCourse(context.Background(),
		connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: strings.Repeat("é", 201)}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	var cerr *connect.Error
	require.ErrorAs(t, err, &cerr)
	require.Len(t, cerr.Details(), 1)
	v, derr := cerr.Details()[0].Value()
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
	v, derr := cerr.Details()[0].Value()
	require.NoError(t, derr)
	br, ok := v.(*errdetails.BadRequest)
	require.True(t, ok)
	return br.GetFieldViolations()[0].GetField()
}

func TestRPC_ReorderLessons_OK_PassesOrder(t *testing.T) {
	e := newEnv(t, "lessons:write")
	c := e.client(t, uuid.New())
	course, err := c.CreateCourse(context.Background(), connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: "c"}))
	require.NoError(t, err)
	cid := course.Msg.GetCourse().GetId()
	a, err := c.CreateLesson(context.Background(), connect.NewRequest(&lessonsv1.CreateLessonRequest{CourseId: cid, Title: "a"}))
	require.NoError(t, err)
	b, err := c.CreateLesson(context.Background(), connect.NewRequest(&lessonsv1.CreateLessonRequest{CourseId: cid, Title: "b"}))
	require.NoError(t, err)

	res, err := c.ReorderLessons(context.Background(), connect.NewRequest(&lessonsv1.ReorderLessonsRequest{
		CourseId: cid, LessonIds: []string{b.Msg.GetLesson().GetId(), a.Msg.GetLesson().GetId()},
	}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetLessons(), 2)
	assert.Equal(t, "b", res.Msg.GetLessons()[0].GetTitle())
	assert.Equal(t, int32(1), res.Msg.GetLessons()[0].GetPosition())
	assert.Len(t, e.repo.reorderIDs, 2)
}

func TestRPC_ReorderLessons_Duplicate_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	id := uuid.NewString()
	_, err := e.client(t, uuid.New()).ReorderLessons(context.Background(), connect.NewRequest(&lessonsv1.ReorderLessonsRequest{
		CourseId: uuid.NewString(), LessonIds: []string{id, id},
	}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Equal(t, "lesson_ids", fieldOf(t, err))
	assert.Nil(t, e.repo.reorderIDs, "repo must not be called")
}

func TestRPC_ReorderLessons_BadLessonID_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	_, err := e.client(t, uuid.New()).ReorderLessons(context.Background(), connect.NewRequest(&lessonsv1.ReorderLessonsRequest{
		CourseId: uuid.NewString(), LessonIds: []string{"nope"},
	}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Equal(t, "lesson_ids", fieldOf(t, err))
}

func TestRPC_ReorderLessons_Mismatch_InvalidArgument(t *testing.T) {
	e := newEnv(t, "lessons:write")
	e.repo.reorderErr = ErrLessonSetMismatch
	_, err := e.client(t, uuid.New()).ReorderLessons(context.Background(), connect.NewRequest(&lessonsv1.ReorderLessonsRequest{
		CourseId: uuid.NewString(), LessonIds: []string{uuid.NewString()},
	}))
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
	e := newEnvWith(t, []connect.Interceptor{apphttp.RPCLog(slog.New(slog.NewJSONHandler(&buf, nil)))}, "lessons:write")
	anon := lessonsv1connect.NewLessonsServiceClient(http.DefaultClient, e.srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := anon.WatchCourse(ctx, connect.NewRequest(&lessonsv1.WatchCourseRequest{Id: uuid.NewString()}))
	require.NoError(t, err)
	assert.False(t, stream.Receive())
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(stream.Err()))
	assert.Contains(t, buf.String(), `"procedure":"/lessons.v1.LessonsService/WatchCourse"`)
	assert.Contains(t, buf.String(), `"code":"unauthenticated"`)
}

func TestRPC_WatchCourse_UnknownCourse_NotFound(t *testing.T) {
	fastWatch(t)
	e := newEnv(t, "lessons:write")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := e.client(t, uuid.New()).WatchCourse(ctx, connect.NewRequest(&lessonsv1.WatchCourseRequest{Id: uuid.NewString()}))
	require.NoError(t, err)
	assert.False(t, stream.Receive())
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(stream.Err()))
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
	course, err := c.CreateCourse(ctx, connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: "watched"}))
	require.NoError(t, err)
	cid := course.Msg.GetCourse().GetId()

	var cp http.Protocols
	cp.SetUnencryptedHTTP2(true)
	h2c := &http.Client{Transport: &http.Transport{Protocols: &cp}}
	g := lessonsv1connect.NewLessonsServiceClient(h2c, e.srv.URL, connect.WithGRPC(), bearer(token(t, uid, time.Now().Add(time.Hour))))
	stream, err := g.WatchCourse(ctx, connect.NewRequest(&lessonsv1.WatchCourseRequest{Id: cid}))
	require.NoError(t, err)

	require.True(t, stream.Receive(), "first snapshot: %v", stream.Err())
	assert.Equal(t, "watched", stream.Msg().GetCourse().GetTitle())
	assert.Empty(t, stream.Msg().GetLessons())

	_, err = c.CreateLesson(ctx, connect.NewRequest(&lessonsv1.CreateLessonRequest{CourseId: cid, Title: "new"}))
	require.NoError(t, err)
	// An unchanged poll sends nothing, so the next message is the change.
	require.True(t, stream.Receive(), "change: %v", stream.Err())
	require.Len(t, stream.Msg().GetLessons(), 1)
	assert.Equal(t, "new", stream.Msg().GetLessons()[0].GetTitle())
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
	course, err := c.CreateCourse(ctx, connect.NewRequest(&lessonsv1.CreateCourseRequest{Title: "idle"}))
	require.NoError(t, err)

	stream, err := c.WatchCourse(ctx, connect.NewRequest(&lessonsv1.WatchCourseRequest{Id: course.Msg.GetCourse().GetId()}))
	require.NoError(t, err)
	require.True(t, stream.Receive(), "first snapshot: %v", stream.Err())
	first := time.Now()
	require.True(t, stream.Receive(), "keepalive: %v", stream.Err())
	assert.GreaterOrEqual(t, time.Since(first), 80*time.Millisecond, "keepalive came too early")
	assert.Equal(t, "idle", stream.Msg().GetCourse().GetTitle())
	require.NoError(t, stream.Close())
}
