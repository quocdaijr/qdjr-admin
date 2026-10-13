package lessons

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"

	lessonsv1 "github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1/lessonsv1connect"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

const maxTitleRunes = 200

// watchInterval is how often WatchCourse re-reads a course. A var so tests
// can shorten it.
var watchInterval = 2 * time.Second

// watchKeepalive re-sends the current snapshot after this long without a
// message. Measured 2026-10-13: the production edge (Cloudflare in front of
// Render) resets an HTTP/2 stream that carries no data for 60s, while a
// stream with a message every 25s stayed up. Keep this well under 60s.
var watchKeepalive = 30 * time.Second

// maxRequestBytes caps a Connect request body after decompression. connect-go
// v1 reads the body before interceptors run, so this is what stops an
// anonymous caller from making the server buffer an arbitrary upload.
const maxRequestBytes = 1 << 20

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

// NewConnectHandler returns the mount path and http.Handler for the service
// with the options every deployment needs: the given interceptors (auth), a
// request-size cap and panic recovery. main.go and the tests share it so the
// tested handler is the served one.
func NewConnectHandler(repo AdminWriter, interceptors ...connect.Interceptor) (string, http.Handler) {
	return lessonsv1connect.NewLessonsServiceHandler(
		NewHandler(repo),
		connect.WithInterceptors(interceptors...),
		connect.WithReadMaxBytes(maxRequestBytes),
		connect.WithRecover(func(_ context.Context, _ connect.Spec, _ http.Header, p any) error {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("lessons: panic: %v", p))
		}),
	)
}

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
			return nil, invalidField("status", "status must be todo, in_progress or done")
		}
		in.Status = &st
	}
	if req.Msg.Position != nil {
		if *req.Msg.Position < 0 {
			return nil, invalidField("position", "position must be >= 0")
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

// ReorderLessons sets the course's lesson order. lesson_ids must name every
// lesson of the course exactly once; the repository enforces the set match.
func (h *Handler) ReorderLessons(ctx context.Context, req *connect.Request[lessonsv1.ReorderLessonsRequest]) (*connect.Response[lessonsv1.ReorderLessonsResponse], error) {
	courseID, err := parseID(req.Msg.GetCourseId())
	if err != nil {
		return nil, err
	}
	raw := req.Msg.GetLessonIds()
	ids := make([]uuid.UUID, 0, len(raw))
	seen := make(map[uuid.UUID]bool, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, invalidField("lesson_ids", "invalid lesson id")
		}
		if seen[id] {
			return nil, invalidField("lesson_ids", "duplicate lesson id")
		}
		seen[id] = true
		ids = append(ids, id)
	}
	lessons, err := h.repo.ReorderLessons(ctx, courseID, ids)
	if errors.Is(err, ErrLessonSetMismatch) {
		return nil, invalidField("lesson_ids", "lesson_ids must list every lesson of the course exactly once")
	}
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*lessonsv1.Lesson, 0, len(lessons))
	for _, l := range lessons {
		out = append(out, lessonToProto(l))
	}
	return connect.NewResponse(&lessonsv1.ReorderLessonsResponse{Lessons: out}), nil
}

// WatchCourse streams the course with its lessons: one snapshot on subscribe,
// then one each time something changes, until the client goes away or the
// course is deleted (NotFound). Changes are found by re-reading every
// watchInterval and comparing with the last snapshot sent; an unchanged
// snapshot is re-sent after watchKeepalive so idle streams survive the edge.
// ponytail: polling, one GetCourse per watcher per tick; switch to
// LISTEN/NOTIFY if watchers ever outnumber a couple of admin tabs.
func (h *Handler) WatchCourse(ctx context.Context, req *connect.Request[lessonsv1.WatchCourseRequest], stream *connect.ServerStream[lessonsv1.WatchCourseResponse]) error {
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return err
	}
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	var (
		last     *lessonsv1.WatchCourseResponse
		lastSent time.Time
	)
	for {
		c, lessons, err := h.repo.GetCourse(ctx, id)
		if ctx.Err() != nil {
			return ctx.Err() // client went away mid-read
		}
		if err != nil {
			return mapErr(err)
		}
		snap := &lessonsv1.WatchCourseResponse{Course: courseToProto(c), Lessons: make([]*lessonsv1.Lesson, 0, len(lessons))}
		for _, l := range lessons {
			snap.Lessons = append(snap.Lessons, lessonToProto(l))
		}
		changed := last == nil || !proto.Equal(last, snap)
		if changed || time.Since(lastSent) >= watchKeepalive {
			if err := stream.Send(snap); err != nil {
				return err
			}
			last, lastSent = snap, time.Now()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// --- helpers ---

func invalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

// invalidField is invalid plus a google.rpc.BadRequest detail naming the
// field, so a client can put the message next to the input, not in a toast.
func invalidField(field, msg string) error {
	cerr := connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
	detail, err := connect.NewErrorDetail(&errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: field, Description: msg}},
	})
	if err == nil {
		cerr.AddDetail(detail)
	}
	return cerr
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
		return "", invalidField("title", fmt.Sprintf("title must be 1..%d characters", maxTitleRunes))
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
