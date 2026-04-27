package tags

import (
	"bytes"
	"context"
	"encoding/json"
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

const adminTestSecret = "test-secret-test-secret-test-secret"

// resolverStub is a minimal PermissionResolver double for handler tests.
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

// stubAdminRepo is an in-memory AdminWriter for handler tests.
type stubAdminRepo struct {
	items     map[uuid.UUID]Resource
	listResp  []Resource
	listTotal int
	listFilt  AdminListFilter
	createIn  CreateInput
	createErr error
	updateIn  UpdateInput
	updateID  uuid.UUID
	updateErr error
	deleteID  uuid.UUID
	deleteErr error
}

func newStubAdminRepo() *stubAdminRepo {
	return &stubAdminRepo{items: map[uuid.UUID]Resource{}}
}

func (s *stubAdminRepo) List(_ context.Context, f AdminListFilter) ([]Resource, int, error) {
	s.listFilt = f
	if s.listResp != nil {
		return s.listResp, s.listTotal, nil
	}
	out := make([]Resource, 0, len(s.items))
	for _, r := range s.items {
		out = append(out, r)
	}
	return out, len(out), nil
}

func (s *stubAdminRepo) GetByID(_ context.Context, id uuid.UUID) (Resource, error) {
	r, ok := s.items[id]
	if !ok {
		return Resource{}, ErrNotFound
	}
	return r, nil
}

func (s *stubAdminRepo) Create(_ context.Context, in CreateInput) (Resource, error) {
	s.createIn = in
	if s.createErr != nil {
		return Resource{}, s.createErr
	}
	id := uuid.New()
	r := Resource{
		ID:          id,
		Slug:        in.Slug,
		Name:        in.Name,
		Description: in.Description,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	s.items[id] = r
	return r, nil
}

func (s *stubAdminRepo) Update(_ context.Context, id uuid.UUID, in UpdateInput) (Resource, error) {
	s.updateID = id
	s.updateIn = in
	if s.updateErr != nil {
		return Resource{}, s.updateErr
	}
	r, ok := s.items[id]
	if !ok {
		return Resource{}, ErrNotFound
	}
	if in.Slug != nil {
		r.Slug = *in.Slug
	}
	if in.Name != nil {
		r.Name = *in.Name
	}
	if in.Description != nil {
		r.Description = in.Description
	}
	s.items[id] = r
	return r, nil
}

func (s *stubAdminRepo) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteID = id
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}

type adminEnv struct {
	router *gin.Engine
	repo   *stubAdminRepo
	res    *resolverStub
}

func newAdminEnv(perms ...string) *adminEnv {
	gin.SetMode(gin.TestMode)
	repo := newStubAdminRepo()
	res := &resolverStub{role: "editor", perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)
	return &adminEnv{router: r, repo: repo, res: res}
}

func (e *adminEnv) do(t *testing.T, uid uuid.UUID, method, path string, body any) *httptest.ResponseRecorder {
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
	signed, _ := tok.SignedString([]byte(adminTestSecret))
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func TestAdminTags_List_OK(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodGet, "/v1/admin/tags?page=2&perPage=5&q=foo", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 2, env.repo.listFilt.Page)
	assert.Equal(t, 5, env.repo.listFilt.PerPage)
	assert.Equal(t, "foo", env.repo.listFilt.Q)
}

func TestAdminTags_List_Forbidden(t *testing.T) {
	env := newAdminEnv()
	rr := env.do(t, uuid.New(), http.MethodGet, "/v1/admin/tags", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminTags_Create_OK_AutoSlug(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/tags", map[string]any{
		"name": "Hello World",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "Hello World", env.repo.createIn.Name)
	assert.Equal(t, "hello-world", env.repo.createIn.Slug)
}

func TestAdminTags_Create_OK_ExplicitSlug(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/tags", map[string]any{
		"name": "Hello",
		"slug": "custom-slug",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "custom-slug", env.repo.createIn.Slug)
}

func TestAdminTags_Create_BadName(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/tags", map[string]any{
		"name": "",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminTags_Create_InvalidSlug(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/tags", map[string]any{
		"name": "Hi",
		"slug": "Bad Slug!",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminTags_Create_SlugConflict(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	env.repo.createErr = ErrSlugConflict
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/tags", map[string]any{
		"name": "Whatever",
	})
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestAdminTags_Get_OK(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	id := uuid.New()
	env.repo.items[id] = Resource{ID: id, Slug: "x", Name: "X"}

	rr := env.do(t, uuid.New(), http.MethodGet, "/v1/admin/tags/"+id.String(), nil)
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestAdminTags_Get_NotFound(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodGet, "/v1/admin/tags/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAdminTags_Get_BadID(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodGet, "/v1/admin/tags/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminTags_Patch_OK(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	id := uuid.New()
	env.repo.items[id] = Resource{ID: id, Slug: "x", Name: "Old"}

	rr := env.do(t, uuid.New(), http.MethodPatch, "/v1/admin/tags/"+id.String(), map[string]any{
		"name": "New",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.updateIn.Name)
	assert.Equal(t, "New", *env.repo.updateIn.Name)
}

func TestAdminTags_Patch_SlugConflict(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	id := uuid.New()
	env.repo.items[id] = Resource{ID: id, Slug: "x", Name: "X"}
	env.repo.updateErr = ErrSlugConflict

	rr := env.do(t, uuid.New(), http.MethodPatch, "/v1/admin/tags/"+id.String(), map[string]any{
		"slug": "taken",
	})
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestAdminTags_Patch_NotFound(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodPatch, "/v1/admin/tags/"+uuid.New().String(), map[string]any{
		"name": "x",
	})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAdminTags_Delete_OK(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	id := uuid.New()
	env.repo.items[id] = Resource{ID: id, Slug: "x", Name: "X"}

	rr := env.do(t, uuid.New(), http.MethodDelete, "/v1/admin/tags/"+id.String(), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminTags_Delete_InUse(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	id := uuid.New()
	env.repo.items[id] = Resource{ID: id, Slug: "x", Name: "X"}
	env.repo.deleteErr = ErrInUse

	rr := env.do(t, uuid.New(), http.MethodDelete, "/v1/admin/tags/"+id.String(), nil)
	require.Equal(t, http.StatusConflict, rr.Code)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "IN_USE", body.Error.Code)
}

func TestAdminTags_Delete_NotFound(t *testing.T) {
	env := newAdminEnv("taxonomy:write")
	rr := env.do(t, uuid.New(), http.MethodDelete, "/v1/admin/tags/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAdminTags_MissingAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newStubAdminRepo()
	res := &resolverStub{role: "editor", perms: map[string]bool{"taxonomy:write": true}}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/tags", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
