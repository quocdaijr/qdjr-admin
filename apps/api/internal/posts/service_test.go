package posts

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolverStub is a minimal PermissionResolver double for service tests.
type resolverStub struct {
	role  string
	perms map[string]bool
}

func (r *resolverStub) Role(_ context.Context, _ uuid.UUID) (string, error) {
	return r.role, nil
}
func (r *resolverStub) Permissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	out := make([]string, 0, len(r.perms))
	for k := range r.perms {
		out = append(out, k)
	}
	return out, nil
}
func (r *resolverStub) Can(_ context.Context, _ uuid.UUID, p string) (bool, error) {
	return r.perms[p], nil
}

func TestRequireOwnedOrElevated(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	post := AdminPost{CreatedBy: &owner}

	t.Run("elevated user (posts:read:all) always allowed", func(t *testing.T) {
		res := &resolverStub{role: "editor", perms: map[string]bool{"posts:read:all": true}}
		err := requireOwnedOrElevated(context.Background(), res, other, post)
		require.NoError(t, err)
	})

	t.Run("author owns post → allowed", func(t *testing.T) {
		res := &resolverStub{role: "author", perms: map[string]bool{}}
		err := requireOwnedOrElevated(context.Background(), res, owner, post)
		require.NoError(t, err)
	})

	t.Run("author of another post → forbidden", func(t *testing.T) {
		res := &resolverStub{role: "author", perms: map[string]bool{}}
		err := requireOwnedOrElevated(context.Background(), res, other, post)
		assert.ErrorIs(t, err, ErrForbidden)
	})

	t.Run("nil created_by + non-elevated → forbidden", func(t *testing.T) {
		res := &resolverStub{role: "author", perms: map[string]bool{}}
		err := requireOwnedOrElevated(context.Background(), res, owner, AdminPost{})
		assert.ErrorIs(t, err, ErrForbidden)
	})
}
