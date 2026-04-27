// Package users provides admin user-management types and helpers.
//
// The admin endpoints under /v1/admin/users join auth.users with
// public.user_roles → public.roles to expose a flat user-with-role shape.
package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a user id is not found.
var ErrNotFound = errors.New("users: not found")

// ErrLastSuperAdmin is returned when an operation would remove the last
// remaining super_admin (self-protection guardrail).
var ErrLastSuperAdmin = errors.New("users: cannot demote/remove last super_admin")

// AllowedRoles is the canonical set of role names that can be assigned via
// the admin endpoints. Kept in sync with public.roles seed data.
var AllowedRoles = []string{"super_admin", "editor", "author"}

// IsAllowedRole reports whether role is one of AllowedRoles.
func IsAllowedRole(role string) bool {
	for _, r := range AllowedRoles {
		if r == role {
			return true
		}
	}
	return false
}

// User is the admin response shape: a flattened auth.users + role row. Users
// without a row in user_roles have Role == nil and AssignedAt == nil.
type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	Role         *string    `json:"role"`
	LastSignInAt *time.Time `json:"last_sign_in_at"`
	CreatedAt    time.Time  `json:"created_at"`
	AssignedAt   *time.Time `json:"assigned_at"`
}

// ListFilter narrows admin list queries.
type ListFilter struct {
	Page    int
	PerPage int
}

// AdminWriter is the contract the admin handler depends on. Defined as an
// interface so handler tests can use a stub without a database.
type AdminWriter interface {
	List(ctx context.Context, f ListFilter) ([]User, int, error)
	Get(ctx context.Context, id uuid.UUID) (User, error)
	SetRole(ctx context.Context, userID uuid.UUID, role string) error
	IsLastSuperAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
	DeleteRole(ctx context.Context, userID uuid.UUID) error
}

// SupabaseAdmin is the contract for Supabase Auth Admin API operations the
// handler depends on. Defined as an interface so handler tests can use a stub.
type SupabaseAdmin interface {
	EnsureUser(ctx context.Context, email, password string) (uuid.UUID, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}
