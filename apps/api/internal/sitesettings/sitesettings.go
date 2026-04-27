// Package sitesettings implements the public-facing Site Settings API endpoint.
//
// public.site_settings is a singleton row (id=1) that holds site-wide UI
// configuration. The public endpoint omits admin-only fields like
// contact_email — those are reachable only via the authenticated admin API.
package sitesettings

import "context"

// Public is the public response shape for site_settings.
//
// Notably this struct does NOT contain `contact_email`: that field is treated
// as admin-only PII and must not leak through the unauthenticated endpoint.
type Public struct {
	SiteTitle       string            `json:"site_title"`
	SiteDescription *string           `json:"site_description"`
	FooterText      *string           `json:"footer_text"`
	SocialLinks     map[string]string `json:"social_links"`
}

// Reader is the read-only contract handlers depend on.
type Reader interface {
	GetPublic(ctx context.Context) (Public, error)
}
