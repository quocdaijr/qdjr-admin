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
