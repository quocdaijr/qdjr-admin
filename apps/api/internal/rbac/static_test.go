package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRoleStore struct {
	roleByUser map[uuid.UUID]string
}

func (f *fakeRoleStore) RoleForUser(_ context.Context, u uuid.UUID) (string, error) {
	if r, ok := f.roleByUser[u]; ok {
		return r, nil
	}
	return "", ErrNoRole
}

func TestStatic_Can_SuperAdminAlwaysAllowed(t *testing.T) {
	u := uuid.New()
	r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "super_admin"}})
	ok, err := r.Can(context.Background(), u, "users:manage")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestStatic_Can_AuthorCannotPublish(t *testing.T) {
	u := uuid.New()
	r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "author"}})
	ok, err := r.Can(context.Background(), u, "posts:publish")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStatic_Can_EditorCannotManageUsers(t *testing.T) {
	u := uuid.New()
	r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "editor"}})
	ok, err := r.Can(context.Background(), u, "users:manage")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStatic_Permissions_AuthorList(t *testing.T) {
	u := uuid.New()
	r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{u: "author"}})
	perms, err := r.Permissions(context.Background(), u)
	require.NoError(t, err)
	assert.Contains(t, perms, "posts:write")
	assert.NotContains(t, perms, "posts:publish")
}

func TestStatic_Role_Unknown(t *testing.T) {
	r := NewStatic(&fakeRoleStore{roleByUser: map[uuid.UUID]string{}})
	_, err := r.Role(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrNoRole)
}
