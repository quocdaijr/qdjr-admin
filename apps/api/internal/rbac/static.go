package rbac

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

// rolePermissions is the spec's RBAC matrix collapsed to a map.
// Author "own only" cases are enforced separately by RequireOwnership middleware.
var rolePermissions = map[string][]string{
	"super_admin": {
		"posts:read:all", "posts:write", "posts:publish",
		"taxonomy:write",
		"media:write",
		"profile:write", "settings:write",
		"contact:read", "contact:write",
		"users:manage",
	},
	"editor": {
		"posts:read:all", "posts:write", "posts:publish",
		"taxonomy:write",
		"media:write",
		"profile:write", "settings:write",
		"contact:read", "contact:write",
	},
	"author": {
		"posts:write", // ownership enforced by middleware
		"media:write", // ownership enforced by middleware
	},
}

type staticResolver struct{ store RoleStore }

// NewStatic builds a resolver backed by the hardcoded matrix above.
func NewStatic(store RoleStore) PermissionResolver { return &staticResolver{store: store} }

func (r *staticResolver) Role(ctx context.Context, userID uuid.UUID) (string, error) {
	return r.store.RoleForUser(ctx, userID)
}

func (r *staticResolver) Permissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	role, err := r.store.RoleForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	perms := rolePermissions[role]
	out := make([]string, len(perms))
	copy(out, perms)
	return out, nil
}

func (r *staticResolver) Can(ctx context.Context, userID uuid.UUID, perm string) (bool, error) {
	role, err := r.store.RoleForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.Contains(rolePermissions[role], perm), nil
}
