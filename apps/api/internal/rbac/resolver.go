package rbac

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrNoRole indicates the caller has no row in user_roles.
var ErrNoRole = errors.New("no role assigned")

// PermissionResolver decides whether a user can perform a permission.
// Future implementations may swap a static map for a DB-backed store.
type PermissionResolver interface {
	Can(ctx context.Context, userID uuid.UUID, perm string) (bool, error)
	Permissions(ctx context.Context, userID uuid.UUID) ([]string, error)
	Role(ctx context.Context, userID uuid.UUID) (string, error)
}

// RoleStore returns the role name for a user (or ErrNoRole).
type RoleStore interface {
	RoleForUser(ctx context.Context, userID uuid.UUID) (string, error)
}
