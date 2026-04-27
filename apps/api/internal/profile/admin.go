package profile

import (
	"context"

	"github.com/google/uuid"
)

// UpdateInput is the payload for AdminRepository.Update. All fields are
// optional; nil pointers mean "do not change".
//
// AvatarID uses **uuid.UUID to distinguish three states: missing key (nil
// pointer-to-pointer), explicit null (non-nil pointer wrapping nil), and a
// real UUID. SocialLinks uses *map[string]string so a present-but-empty body
// clears the map while a missing key leaves it untouched.
type UpdateInput struct {
	FullName    *string
	Bio         *string
	AvatarID    **uuid.UUID
	Tagline     *string
	SocialLinks *map[string]string
	Location    *string
	Email       *string
	UpdatedBy   uuid.UUID
}

// AdminWriter is the contract the admin handler depends on. Defined as an
// interface so tests can use a stub without a database.
type AdminWriter interface {
	Get(ctx context.Context) (Profile, error)
	Update(ctx context.Context, in UpdateInput) (Profile, error)
}
