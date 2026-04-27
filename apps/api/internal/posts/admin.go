package posts

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrSlugConflict is returned when a slug already exists.
var ErrSlugConflict = errors.New("posts: slug conflict")

// AdminPost is the admin response shape: includes status, ownership, and full
// taxonomy projections (all categories + all tags), unlike the public Post.
type AdminPost struct {
	ID              uuid.UUID  `json:"id"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Excerpt         *string    `json:"excerpt"`
	Content         string     `json:"content"`
	Status          string     `json:"status"`
	ThumbnailID     *uuid.UUID `json:"thumbnail_id"`
	OGImageID       *uuid.UUID `json:"og_image_id"`
	MetaTitle       *string    `json:"meta_title"`
	MetaDescription *string    `json:"meta_description"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedBy       *uuid.UUID `json:"created_by"`
	UpdatedBy       *uuid.UUID `json:"updated_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Categories      []Category `json:"categories"`
	Tags            []Tag      `json:"tags"`
}

// AdminListFilter narrows admin list queries.
//
// Status: "" or "all" → no status filter. Otherwise must be one of the
// post_status enum values ("draft", "published", "archived").
type AdminListFilter struct {
	Page      int
	PerPage   int
	Status    string
	Q         string
	CreatedBy *uuid.UUID // when set, only posts authored by this user
}

// CreateInput is the payload for Create. CategoryIDs and TagIDs may be nil
// (no pivots inserted). Status defaults to "draft" when empty.
type CreateInput struct {
	Title           string
	Slug            string // optional; auto from title if empty
	Excerpt         *string
	Content         string
	Status          string
	ThumbnailID     *uuid.UUID
	OGImageID       *uuid.UUID
	MetaTitle       *string
	MetaDescription *string
	CategoryIDs     []uuid.UUID
	TagIDs          []uuid.UUID
	CreatedBy       uuid.UUID
}

// UpdateInput is the payload for Update. All fields are optional. Nil pointers
// mean "do not change". For CategoryIDs / TagIDs: nil means leave pivots alone;
// non-nil (even empty slice) means REPLACE pivots.
type UpdateInput struct {
	Title           *string
	Slug            *string
	Excerpt         *string
	Content         *string
	Status          *string
	ThumbnailID     *uuid.UUID
	OGImageID       *uuid.UUID
	MetaTitle       *string
	MetaDescription *string
	CategoryIDs     *[]uuid.UUID
	TagIDs          *[]uuid.UUID
	UpdatedBy       uuid.UUID
}

// AdminWriter is the contract the admin handler depends on. Defined as an
// interface so tests can use a stub without a database.
type AdminWriter interface {
	List(ctx context.Context, f AdminListFilter) ([]AdminPost, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (AdminPost, error)
	Create(ctx context.Context, in CreateInput) (AdminPost, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (AdminPost, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetPublished(ctx context.Context, id uuid.UUID, publish bool, updatedBy uuid.UUID) (AdminPost, error)
}
