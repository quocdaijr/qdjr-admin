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
