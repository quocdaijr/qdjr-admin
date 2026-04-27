package categories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads categories from Postgres.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const selectColumns = `id, slug, name, description, created_at, updated_at`

// List returns all categories ordered by name. No pagination.
func (r *Repository) List(ctx context.Context) ([]Resource, error) {
	const q = `SELECT ` + selectColumns + ` FROM public.categories ORDER BY name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("categories list: %w", err)
	}
	defer rows.Close()

	out := make([]Resource, 0)
	for rows.Next() {
		var c Resource
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("categories list scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("categories list iterate: %w", err)
	}
	return out, nil
}

// GetBySlug returns a single category or ErrNotFound.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (Resource, error) {
	const q = `SELECT ` + selectColumns + ` FROM public.categories WHERE slug = $1 LIMIT 1`
	var c Resource
	err := r.pool.QueryRow(ctx, q, slug).Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resource{}, ErrNotFound
	}
	if err != nil {
		return Resource{}, fmt.Errorf("categories slug: %w", err)
	}
	return c, nil
}
