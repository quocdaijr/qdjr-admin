package tags

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads tags from Postgres.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const selectColumns = `id, slug, name, description, created_at, updated_at`

// List returns all tags ordered by name. No pagination.
func (r *Repository) List(ctx context.Context) ([]Resource, error) {
	const q = `SELECT ` + selectColumns + ` FROM public.tags ORDER BY name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("tags list: %w", err)
	}
	defer rows.Close()

	out := make([]Resource, 0)
	for rows.Next() {
		var t Resource
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("tags list scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tags list iterate: %w", err)
	}
	return out, nil
}

// GetBySlug returns a single tag or ErrNotFound.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (Resource, error) {
	const q = `SELECT ` + selectColumns + ` FROM public.tags WHERE slug = $1 LIMIT 1`
	var t Resource
	err := r.pool.QueryRow(ctx, q, slug).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resource{}, ErrNotFound
	}
	if err != nil {
		return Resource{}, fmt.Errorf("tags slug: %w", err)
	}
	return t, nil
}
