package lessons

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

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

	// The page and the count are independent reads on separate pooled
	// connections, so run them concurrently; errgroup cancels the sibling on
	// the first error. For one admin on free Neon the win is tiny, but it is
	// the right shape once the dataset or the round-trip grows.
	var (
		out   []Course
		total int
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		const listSQL = courseSelect + `
        WHERE ($1::text = '' OR c.title ILIKE '%' || $1 || '%')
        GROUP BY c.id
        ORDER BY c.updated_at DESC
        LIMIT $2 OFFSET $3`
		rows, err := r.pool.Query(gctx, listSQL, q, f.PerPage, (f.Page-1)*f.PerPage)
		if err != nil {
			return fmt.Errorf("lessons list courses: %w", err)
		}
		defer rows.Close()
		page := make([]Course, 0, f.PerPage)
		for rows.Next() {
			c, err := scanCourse(rows)
			if err != nil {
				return fmt.Errorf("lessons list courses scan: %w", err)
			}
			page = append(page, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("lessons list courses iterate: %w", err)
		}
		out = page
		return nil
	})
	g.Go(func() error {
		const countSQL = `SELECT count(*) FROM public.courses WHERE ($1::text = '' OR title ILIKE '%' || $1 || '%')`
		if err := r.pool.QueryRow(gctx, countSQL, q).Scan(&total); err != nil {
			return fmt.Errorf("lessons count courses: %w", err)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, 0, err
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

// ReorderLessons rewrites positions 1..n in one transaction. The course row
// is locked so two reorders of the same course serialise, and the ids must be
// exactly the course's lessons (no missing, extra, foreign or repeated ids),
// otherwise ErrLessonSetMismatch and nothing changes.
func (r *AdminRepository) ReorderLessons(ctx context.Context, courseID uuid.UUID, ids []uuid.UUID) ([]Lesson, error) {
	idStrs := make([]string, 0, len(ids))
	for _, id := range ids {
		idStrs = append(idStrs, id.String())
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("lessons reorder begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	var locked int
	err = tx.QueryRow(ctx, `SELECT 1 FROM public.courses WHERE id = $1 FOR UPDATE`, courseID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lessons reorder lock course: %w", err)
	}

	// total lessons in the course, and how many of them the request names.
	// Equal to len(ids) on both counts means the sets match exactly.
	var total, named int
	const countSQL = `
        SELECT count(*), count(*) FILTER (WHERE id = ANY($2::uuid[]))
        FROM public.lessons WHERE course_id = $1`
	if err := tx.QueryRow(ctx, countSQL, courseID, idStrs).Scan(&total, &named); err != nil {
		return nil, fmt.Errorf("lessons reorder count: %w", err)
	}
	if total != len(ids) || named != len(ids) {
		return nil, ErrLessonSetMismatch
	}

	const updateSQL = `
        UPDATE public.lessons l
        SET position = v.ord::int, updated_at = now()
        FROM unnest($2::uuid[]) WITH ORDINALITY AS v(id, ord)
        WHERE l.id = v.id AND l.course_id = $1`
	if _, err := tx.Exec(ctx, updateSQL, courseID, idStrs); err != nil {
		return nil, fmt.Errorf("lessons reorder update: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("lessons reorder commit: %w", err)
	}

	_, lessons, err := r.GetCourse(ctx, courseID)
	return lessons, err
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
