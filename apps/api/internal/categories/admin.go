package categories

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrSlugConflict is returned when a slug already exists.
var ErrSlugConflict = errors.New("categories: slug conflict")

// ErrInUse is returned when a delete is rejected because of dependent rows
// (post_categories pivot references).
var ErrInUse = errors.New("categories: in use")

// AdminListFilter narrows admin list queries.
type AdminListFilter struct {
	Page    int
	PerPage int
	Q       string // ILIKE on name
}

// CreateInput is the payload for Create. Slug is optional; auto-generated from
// name when empty.
type CreateInput struct {
	Slug        string
	Name        string
	Description *string
}

// UpdateInput is the payload for Update. All fields are optional. Nil pointers
// mean "do not change".
type UpdateInput struct {
	Slug        *string
	Name        *string
	Description *string
}

// AdminWriter is the contract the admin handler depends on. Defined as an
// interface so tests can use a stub without a database.
type AdminWriter interface {
	List(ctx context.Context, f AdminListFilter) ([]Resource, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (Resource, error)
	Create(ctx context.Context, in CreateInput) (Resource, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Resource, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
