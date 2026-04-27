package users

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

const usersTestSecret = "test-secret-test-secret-test-secret"

// resolverStub grants users:manage to any caller.
type resolverStub struct {
	perms map[string]bool
}

func newResolverStub(perms ...string) *resolverStub {
	r := &resolverStub{perms: map[string]bool{}}
	for _, p := range perms {
		r.perms[p] = true
	}
	return r
}

func (r *resolverStub) Role(_ context.Context, _ uuid.UUID) (string, error) {
	return "super_admin", nil
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

// stubRepo is an in-memory AdminWriter for handler tests.
type stubRepo struct {
	users        map[uuid.UUID]User
	listResp     []User
	listTotal    int
	listErr      error
	setRoleErr   error
	setRoleCalls []struct {
		id   uuid.UUID
		role string
	}
	isLastResp map[uuid.UUID]bool
	isLastErr  error
	getErr     error
	deleteErr  error
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		users:      map[uuid.UUID]User{},
		isLastResp: map[uuid.UUID]bool{},
	}
}

func (s *stubRepo) List(_ context.Context, _ ListFilter) ([]User, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	if s.listResp != nil {
		return s.listResp, s.listTotal, nil
	}
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out, len(out), nil
}

func (s *stubRepo) Get(_ context.Context, id uuid.UUID) (User, error) {
	if s.getErr != nil {
		return User{}, s.getErr
	}
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (s *stubRepo) SetRole(_ context.Context, id uuid.UUID, role string) error {
	s.setRoleCalls = append(s.setRoleCalls, struct {
		id   uuid.UUID
		role string
	}{id, role})
	if s.setRoleErr != nil {
		return s.setRoleErr
	}
	u := s.users[id]
	u.ID = id
	r := role
	u.Role = &r
	now := time.Now().UTC()
	u.AssignedAt = &now
	if u.Email == "" {
		u.Email = id.String() + "@example.test"
	}
	s.users[id] = u
	return nil
}

func (s *stubRepo) IsLastSuperAdmin(_ context.Context, id uuid.UUID) (bool, error) {
	if s.isLastErr != nil {
		return false, s.isLastErr
	}
	return s.isLastResp[id], nil
}

func (s *stubRepo) DeleteRole(_ context.Context, id uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	u, ok := s.users[id]
	if ok {
		u.Role = nil
		u.AssignedAt = nil
		s.users[id] = u
	}
	return nil
}

// stubSupa is an in-memory SupabaseAdmin double.
type stubSupa struct {
	ensureCalls []struct {
		email    string
		password string
	}
	deleteCalls   []uuid.UUID
	ensureResp    map[string]uuid.UUID
	ensureErr     error
	deleteErr     error
	requireCreate bool // if true, EnsureUser errors when password == ""
}

func newStubSupa() *stubSupa {
	return &stubSupa{ensureResp: map[string]uuid.UUID{}}
}

func (s *stubSupa) EnsureUser(_ context.Context, email, password string) (uuid.UUID, error) {
	s.ensureCalls = append(s.ensureCalls, struct {
		email    string
		password string
	}{email, password})
	if s.ensureErr != nil {
		return uuid.Nil, s.ensureErr
	}
	if id, ok := s.ensureResp[email]; ok {
		return id, nil
	}
	if password == "" && s.requireCreate {
		return uuid.Nil, errors.New("password required")
	}
	id := uuid.New()
	s.ensureResp[email] = id
	return id, nil
}

func (s *stubSupa) DeleteUser(_ context.Context, id uuid.UUID) error {
	s.deleteCalls = append(s.deleteCalls, id)
	return s.deleteErr
}

type usersEnv struct {
	router *gin.Engine
	repo   *stubRepo
	supa   *stubSupa
	res    *resolverStub
}

func newUsersEnv(perms ...string) *usersEnv {
	gin.SetMode(gin.TestMode)
	repo := newStubRepo()
	supa := newStubSupa()
	res := newResolverStub(perms...)
	v := auth.NewHS256Verifier([]byte(usersTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, supa, res)
	return &usersEnv{router: r, repo: repo, supa: supa, res: res}
}

func (e *usersEnv) do(t *testing.T, uid uuid.UUID, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uid.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, _ := tok.SignedString([]byte(usersTestSecret))
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func TestAdminUsers_List_OK(t *testing.T) {
	env := newUsersEnv("users:manage")
	id := uuid.New()
	role := "editor"
	env.repo.listResp = []User{{ID: id, Email: "x@example.com", Role: &role}}
	env.repo.listTotal = 1

	rr := env.do(t, uuid.New(), http.MethodGet, "/v1/admin/users?page=2&perPage=5", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data []User         `json:"data"`
		Meta apphttp.Meta   `json:"meta"`
		Err  map[string]any `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, 2, body.Meta.Page)
	assert.Equal(t, 5, body.Meta.PerPage)
	assert.Equal(t, 1, body.Meta.Total)
	require.Len(t, body.Data, 1)
	assert.Equal(t, id, body.Data[0].ID)
}

func TestAdminUsers_Invite_NewUser_Created(t *testing.T) {
	env := newUsersEnv("users:manage")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/users", map[string]any{
		"email":    "new@example.com",
		"role":     "editor",
		"password": "S3cret-Pass!",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	require.Len(t, env.supa.ensureCalls, 1)
	assert.Equal(t, "new@example.com", env.supa.ensureCalls[0].email)
	assert.Equal(t, "S3cret-Pass!", env.supa.ensureCalls[0].password)
	require.Len(t, env.repo.setRoleCalls, 1)
	assert.Equal(t, "editor", env.repo.setRoleCalls[0].role)

	var body struct {
		Data User `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Data.Role)
	assert.Equal(t, "editor", *body.Data.Role)
}

func TestAdminUsers_Invite_ExistingUser_NoPassword_OnlySetRole(t *testing.T) {
	env := newUsersEnv("users:manage")
	existing := uuid.New()
	env.supa.ensureResp["existing@example.com"] = existing

	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/users", map[string]any{
		"email": "existing@example.com",
		"role":  "author",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	require.Len(t, env.supa.ensureCalls, 1)
	assert.Equal(t, "", env.supa.ensureCalls[0].password)
	require.Len(t, env.repo.setRoleCalls, 1)
	assert.Equal(t, existing, env.repo.setRoleCalls[0].id)
	assert.Equal(t, "author", env.repo.setRoleCalls[0].role)
}

func TestAdminUsers_Invite_MissingEmail(t *testing.T) {
	env := newUsersEnv("users:manage")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/users", map[string]any{
		"role": "editor",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminUsers_Invite_BadRole(t *testing.T) {
	env := newUsersEnv("users:manage")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/users", map[string]any{
		"email": "ok@example.com",
		"role":  "godking",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminUsers_Invite_BadEmail(t *testing.T) {
	env := newUsersEnv("users:manage")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/users", map[string]any{
		"email": "not-an-email",
		"role":  "editor",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminUsers_PatchRole_OtherUser_OK(t *testing.T) {
	env := newUsersEnv("users:manage")
	caller := uuid.New()
	target := uuid.New()
	env.repo.users[target] = User{ID: target, Email: "t@example.com"}

	rr := env.do(t, caller, http.MethodPatch, "/v1/admin/users/"+target.String()+"/role", map[string]any{
		"role": "editor",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.Len(t, env.repo.setRoleCalls, 1)
	assert.Equal(t, target, env.repo.setRoleCalls[0].id)
	assert.Equal(t, "editor", env.repo.setRoleCalls[0].role)
}

func TestAdminUsers_PatchRole_DemoteSelf_LastSuperAdmin_Conflict(t *testing.T) {
	env := newUsersEnv("users:manage")
	caller := uuid.New()
	env.repo.users[caller] = User{ID: caller, Email: "me@example.com"}
	env.repo.isLastResp[caller] = true

	rr := env.do(t, caller, http.MethodPatch, "/v1/admin/users/"+caller.String()+"/role", map[string]any{
		"role": "editor",
	})
	require.Equal(t, http.StatusConflict, rr.Code)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "LAST_SUPER_ADMIN", body.Error.Code)
	assert.Empty(t, env.repo.setRoleCalls, "must not call SetRole when blocked")
}

func TestAdminUsers_Delete_Self_LastSuperAdmin_Conflict(t *testing.T) {
	env := newUsersEnv("users:manage")
	caller := uuid.New()
	env.repo.users[caller] = User{ID: caller}
	env.repo.isLastResp[caller] = true

	rr := env.do(t, caller, http.MethodDelete, "/v1/admin/users/"+caller.String(), nil)
	require.Equal(t, http.StatusConflict, rr.Code)
	assert.Empty(t, env.supa.deleteCalls, "must not call Supabase delete when blocked")
}

func TestAdminUsers_Delete_OtherUser_NoContent(t *testing.T) {
	env := newUsersEnv("users:manage")
	caller := uuid.New()
	target := uuid.New()
	env.repo.users[target] = User{ID: target}

	rr := env.do(t, caller, http.MethodDelete, "/v1/admin/users/"+target.String(), nil)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Len(t, env.supa.deleteCalls, 1)
	assert.Equal(t, target, env.supa.deleteCalls[0])
}

func TestAdminUsers_Forbidden_WithoutPermission(t *testing.T) {
	env := newUsersEnv() // no perms
	rr := env.do(t, uuid.New(), http.MethodGet, "/v1/admin/users", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
