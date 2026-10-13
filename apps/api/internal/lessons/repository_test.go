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

func (f *fixture) lessonsOf(c Course, titles ...string) []Lesson {
	f.t.Helper()
	out := make([]Lesson, 0, len(titles))
	for _, title := range titles {
		l, err := f.repo.CreateLesson(f.ctx, CreateLessonInput{CourseID: c.ID, Title: title})
		require.NoError(f.t, err)
		out = append(out, l)
	}
	return out
}

func titlesOf(ls []Lesson) []string {
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.Title)
	}
	return out
}

func TestRepo_ReorderLessons_Reverses(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-reorder-" + uuid.NewString()[:8])
	ls := f.lessonsOf(c, "a", "b", "c")

	got, err := f.repo.ReorderLessons(f.ctx, c.ID, []uuid.UUID{ls[2].ID, ls[1].ID, ls[0].ID})
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "b", "a"}, titlesOf(got))
	for i, l := range got {
		assert.Equal(t, i+1, l.Position)
	}

	_, stored, err := f.repo.GetCourse(f.ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "b", "a"}, titlesOf(stored))
}

func TestRepo_ReorderLessons_Mismatch_LeavesOrder(t *testing.T) {
	f := newFixture(t)
	c := f.course("repo-reorder-bad-" + uuid.NewString()[:8])
	ls := f.lessonsOf(c, "a", "b")
	other := f.course("repo-reorder-other-" + uuid.NewString()[:8])
	foreign := f.lessonsOf(other, "x")

	for name, ids := range map[string][]uuid.UUID{
		"missing one": {ls[1].ID},
		"foreign id":  {ls[1].ID, foreign[0].ID},
		"extra id":    {ls[1].ID, ls[0].ID, foreign[0].ID},
	} {
		_, err := f.repo.ReorderLessons(f.ctx, c.ID, ids)
		assert.ErrorIs(t, err, ErrLessonSetMismatch, name)
	}
	_, stored, err := f.repo.GetCourse(f.ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, titlesOf(stored))
}

func TestRepo_ReorderLessons_UnknownCourse_NotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.repo.ReorderLessons(f.ctx, uuid.New(), nil)
	assert.ErrorIs(t, err, ErrNotFound)
}
