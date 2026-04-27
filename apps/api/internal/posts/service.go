package posts

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// ErrForbidden is returned when an author attempts to access another user's post.
var ErrForbidden = errors.New("posts: forbidden")

// canAccessAnyPost reports whether the user holds the cross-author read
// permission ("posts:read:all"). Editors and super_admins do; authors do not.
func canAccessAnyPost(ctx context.Context, res rbac.PermissionResolver, uid uuid.UUID) (bool, error) {
	return res.Can(ctx, uid, "posts:read:all")
}

// requireOwnedOrElevated returns nil when the caller may operate on the post:
// they either hold "posts:read:all" OR they are the post's creator.
//
// Used by GET, PATCH, and DELETE handlers to enforce author-role ownership.
func requireOwnedOrElevated(ctx context.Context, res rbac.PermissionResolver, uid uuid.UUID, post AdminPost) error {
	elevated, err := canAccessAnyPost(ctx, res, uid)
	if err != nil {
		return err
	}
	if elevated {
		return nil
	}
	if post.CreatedBy != nil && *post.CreatedBy == uid {
		return nil
	}
	return ErrForbidden
}
