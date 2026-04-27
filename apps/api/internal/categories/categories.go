// Package categories implements the public-facing Categories API endpoints.
package categories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a category cannot be located by slug.
var ErrNotFound = errors.New("categories: not found")

// Resource is the public response shape for a single category.
//
// Description is a pointer so it serializes as JSON null when absent
// (the column is nullable in Postgres).
type Resource struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Reader is the read-only contract handlers depend on. It exists so handler
// tests can stub the repository without a database.
type Reader interface {
	List(ctx context.Context) ([]Resource, error)
	GetBySlug(ctx context.Context, slug string) (Resource, error)
}
