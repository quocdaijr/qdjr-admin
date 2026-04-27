package media

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a media id is not found.
var ErrNotFound = errors.New("media: not found")

// Allowed mime types for uploads. Mirrors the validation in CreateInput.
var AllowedMimeTypes = []string{
	"image/png",
	"image/jpeg",
	"image/webp",
	"image/gif",
	"image/svg+xml",
}

// MimeExtensions maps allowed mime types to canonical file extensions.
var MimeExtensions = map[string]string{
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/webp":    ".webp",
	"image/gif":     ".gif",
	"image/svg+xml": ".svg",
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

// Storage is the contract for object-storage operations the handler depends on.
type Storage interface {
	SignedUploadURL(ctx context.Context, path string) (string, error)
	Delete(ctx context.Context, path string) error
}
