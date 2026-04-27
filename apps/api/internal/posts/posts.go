// Package posts implements the public-facing Posts API endpoints.
package posts

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a published post cannot be located.
var ErrNotFound = errors.New("posts: not found")

// Thumbnail is the public-facing media reference attached to a post.
// URL is a fully-qualified, browser-loadable URL when the repository is
// configured with a Supabase URL prefix; otherwise it is the raw storage path.
type Thumbnail struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

// Category is the lightweight projection used in public post responses.
type Category struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

// Tag is the lightweight projection used in public post responses.
type Tag struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

// Post is the public response shape for a single post.
//
// Field names match the qdjr frontend contract (see qdjr/plugins/api.ts).
// `location` is intentionally always nil and kept for FE backward compatibility.
type Post struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Excerpt     *string    `json:"excerpt"`
	Content     string     `json:"content"`
	PublishedAt *time.Time `json:"published_at"`
	Thumbnail   *Thumbnail `json:"thumbnail"`
	Location    *string    `json:"location"`
	Category    *Category  `json:"category"`
	Tags        []Tag      `json:"tags"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ListFilter narrows the public post list query.
//
// Page is 1-indexed. PerPage must already be clamped by the caller (handler).
type ListFilter struct {
	Page         int
	PerPage      int
	CategorySlug string // empty = no filter
	TagSlug      string // empty = no filter
	Q            string // empty = no filter; ILIKE against title
}

// Reader is the read-only contract handlers depend on. It exists so handler
// tests can stub the repository without a database.
type Reader interface {
	ListPublished(ctx context.Context, f ListFilter) ([]Post, int, error)
	GetPublishedBySlug(ctx context.Context, slug string) (Post, error)
}
