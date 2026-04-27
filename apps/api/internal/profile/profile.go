// Package profile implements the public-facing Profile API endpoint.
//
// The profile is a singleton row (public.profile, id=1) that surfaces
// site-owner identity for the FE: name, bio, avatar, contact handles.
package profile

import (
	"context"
	"time"
)

// Profile is the public response shape for the singleton profile row.
//
// All optional text columns are returned as JSON null when absent. AvatarURL
// is the resolved public URL (storage prefix + media.storage_path) when an
// avatar is linked, otherwise nil.
type Profile struct {
	ID          int16             `json:"id"`
	FullName    *string           `json:"full_name"`
	Bio         *string           `json:"bio"`
	AvatarURL   *string           `json:"avatar_url"`
	Tagline     *string           `json:"tagline"`
	SocialLinks map[string]string `json:"social_links"`
	Location    *string           `json:"location"`
	Email       *string           `json:"email"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Reader is the read-only contract handlers depend on. Exists so handler
// tests can stub the repository without a database.
type Reader interface {
	Get(ctx context.Context) (Profile, error)
}
