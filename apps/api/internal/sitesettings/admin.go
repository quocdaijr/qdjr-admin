package sitesettings

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Admin is the admin response shape; includes contact_email (which is omitted
// from the public response).
type Admin struct {
	ID              int16             `json:"id"`
	SiteTitle       string            `json:"site_title"`
	SiteDescription *string           `json:"site_description"`
	FooterText      *string           `json:"footer_text"`
	ContactEmail    *string           `json:"contact_email"`
	SocialLinks     map[string]string `json:"social_links"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// UpdateInput is the partial-update payload. Nil pointers mean "do not change".
type UpdateInput struct {
	SiteTitle       *string
	SiteDescription *string
	FooterText      *string
	ContactEmail    *string
	SocialLinks     *map[string]string
	UpdatedBy       uuid.UUID
}

// AdminWriter is the contract the admin handler depends on.
type AdminWriter interface {
	Get(ctx context.Context) (Admin, error)
	Update(ctx context.Context, in UpdateInput) (Admin, error)
}
