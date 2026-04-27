package tags

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

// AdminRepository provides admin CRUD for tags.
type AdminRepository struct {
	pool *pgxpool.Pool
}

// NewAdminRepository constructs an AdminRepository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

// List returns paginated tags filtered by name (ILIKE).
func (r *AdminRepository) List(ctx context.Context, f AdminListFilter) ([]Resource, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}
	q := nullableText(strings.TrimSpace(f.Q))

	const listSQL = `
        SELECT ` + selectColumns + `
        FROM public.tags
        WHERE ($1::text IS NULL OR name ILIKE '%' || $1 || '%')
        ORDER BY name
        LIMIT $2 OFFSET $3`

	offset := (f.Page - 1) * f.PerPage
	rows, err := r.pool.Query(ctx, listSQL, q, f.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("admin tags list: %w", err)
	}
	defer rows.Close()

	out := make([]Resource, 0, f.PerPage)
	for rows.Next() {
		var t Resource
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("admin tags list scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin tags list iterate: %w", err)
	}

	const countSQL = `
        SELECT count(*) FROM public.tags
        WHERE ($1::text IS NULL OR name ILIKE '%' || $1 || '%')`
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, q).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin tags count: %w", err)
	}
	return out, total, nil
}

// GetByID returns a single tag or ErrNotFound.
func (r *AdminRepository) GetByID(ctx context.Context, id uuid.UUID) (Resource, error) {
	const q = `SELECT ` + selectColumns + ` FROM public.tags WHERE id = $1 LIMIT 1`
	var t Resource
	err := r.pool.QueryRow(ctx, q, id).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resource{}, ErrNotFound
	}
	if err != nil {
		return Resource{}, fmt.Errorf("admin tags get: %w", err)
	}
	return t, nil
}

// Create inserts a tag.
func (r *AdminRepository) Create(ctx context.Context, in CreateInput) (Resource, error) {
	id := uuid.New()
	const insert = `
        INSERT INTO public.tags (id, slug, name, description)
        VALUES ($1, $2, $3, $4)`
	if _, err := r.pool.Exec(ctx, insert, id, in.Slug, in.Name, in.Description); err != nil {
		if isUniqueViolation(err) {
			return Resource{}, ErrSlugConflict
		}
		return Resource{}, fmt.Errorf("admin tags insert: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Update applies a partial update. Returns ErrNotFound if no row matches.
func (r *AdminRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Resource, error) {
	sets := make([]string, 0, 4)
	args := make([]any, 0, 4)
	nextArg := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, nextArg))
		args = append(args, v)
		nextArg++
	}
	if in.Slug != nil {
		add("slug", *in.Slug)
	}
	if in.Name != nil {
		add("name", *in.Name)
	}
	if in.Description != nil {
		add("description", *in.Description)
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}
	sets = append(sets, "updated_at = now()")
	stmt := fmt.Sprintf(`UPDATE public.tags SET %s WHERE id = $%d`,
		strings.Join(sets, ", "), nextArg)
	args = append(args, id)
	cmd, err := r.pool.Exec(ctx, stmt, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return Resource{}, ErrSlugConflict
		}
		return Resource{}, fmt.Errorf("admin tags update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return Resource{}, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

// Delete removes a tag. Returns ErrInUse if any post references it,
// ErrNotFound if no row matches.
func (r *AdminRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var refs int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM public.post_tags WHERE tag_id = $1`, id,
	).Scan(&refs); err != nil {
		return fmt.Errorf("admin tags ref-count: %w", err)
	}
	if refs > 0 {
		return fmt.Errorf("tag referenced by %d posts: %w", refs, ErrInUse)
	}
	cmd, err := r.pool.Exec(ctx, `DELETE FROM public.tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("admin tags delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// nullableText returns *string=nil for empty, otherwise a pointer to v.
func nullableText(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
