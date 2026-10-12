// Package media implements the admin media library: uploads through the API
// into object storage (Cloudflare R2, or a local folder in development),
// listing and deletion.
package media

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a media id is not found.
var ErrNotFound = errors.New("media: not found")

// MimeExtensions maps the accepted upload types (detected from the file's
// bytes, never the client's claim) to canonical extensions. SVG is excluded:
// it can carry script.
var MimeExtensions = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// MaxUploadSize is the maximum allowed upload size in bytes (10 MB).
const MaxUploadSize int64 = 10 * 1024 * 1024

// IsAllowedMime reports whether m is in the upload allowlist.
func IsAllowedMime(m string) bool {
	_, ok := MimeExtensions[m]
	return ok
}

// Media is the admin response shape for a public.media row. URL is derived
// from storage_path at scan time using the public-storage prefix.
type Media struct {
	ID          uuid.UUID  `json:"id"`
	Filename    string     `json:"filename"`
	StoragePath string     `json:"storage_path"`
	MimeType    string     `json:"mime_type"`
	Size        int64      `json:"size"`
	Width       *int       `json:"width"`
	Height      *int       `json:"height"`
	AltText     *string    `json:"alt_text"`
	UploadedBy  *uuid.UUID `json:"uploaded_by"`
	CreatedAt   time.Time  `json:"created_at"`
	URL         string     `json:"url"`
}

// CreateInput is the payload for AdminRepository.Create.
type CreateInput struct {
	Filename    string
	StoragePath string
	MimeType    string
	Size        int64
	Width       *int
	Height      *int
	AltText     *string
	UploadedBy  uuid.UUID
}

// AdminListFilter narrows admin list queries.
type AdminListFilter struct {
	Page    int
	PerPage int
}

// AdminWriter is the contract the admin handler depends on. Defined as an
// interface so handler tests can use a stub without a database.
type AdminWriter interface {
	List(ctx context.Context, f AdminListFilter) ([]Media, int, error)
	Get(ctx context.Context, id uuid.UUID) (Media, error)
	Create(ctx context.Context, in CreateInput) (Media, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Storage is the object store the handler writes to. Keys look like
// "media/<uuid>.<ext>"; Delete of a missing key is not an error.
type Storage interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
}
