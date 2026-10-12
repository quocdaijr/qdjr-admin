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
