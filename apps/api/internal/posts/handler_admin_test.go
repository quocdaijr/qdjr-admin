package posts

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

// stubAdminRepo is an in-memory AdminWriter for handler tests.
type stubAdminRepo struct {
	posts          map[uuid.UUID]AdminPost
	listFilter     AdminListFilter
	listResp       []AdminPost
	listTotal      int
	createIn       CreateInput
	createErr      error
	updateIn       UpdateInput
	updateID       uuid.UUID
	updateErr      error
	deleteCalledID uuid.UUID
	publishCalls   []struct {
		id      uuid.UUID
		publish bool
	}
}

func newStubAdminRepo() *stubAdminRepo {
	return &stubAdminRepo{posts: map[uuid.UUID]AdminPost{}}
}

func (s *stubAdminRepo) List(_ context.Context, f AdminListFilter) ([]AdminPost, int, error) {
	s.listFilter = f
	if s.listResp != nil {
		return s.listResp, s.listTotal, nil
	}
	out := make([]AdminPost, 0, len(s.posts))
	for _, p := range s.posts {
		if f.CreatedBy != nil && (p.CreatedBy == nil || *p.CreatedBy != *f.CreatedBy) {
			continue
		}
		out = append(out, p)
	}
	return out, len(out), nil
}

func (s *stubAdminRepo) GetByID(_ context.Context, id uuid.UUID) (AdminPost, error) {
	p, ok := s.posts[id]
	if !ok {
		return AdminPost{}, ErrNotFound
	}
	return p, nil
}

func (s *stubAdminRepo) Create(_ context.Context, in CreateInput) (AdminPost, error) {
	s.createIn = in
	if s.createErr != nil {
		return AdminPost{}, s.createErr
	}
	id := uuid.New()
	by := in.CreatedBy
	p := AdminPost{
		ID:         id,
		Slug:       in.Slug,
		Title:      in.Title,
		Content:    in.Content,
		Status:     in.Status,
		CreatedBy:  &by,
		UpdatedBy:  &by,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
		Categories: []Category{},
		Tags:       []Tag{},
	}
	if in.Status == "published" {
		now := time.Now().UTC()
		p.PublishedAt = &now
	}
	s.posts[id] = p
	return p, nil
}

func (s *stubAdminRepo) Update(_ context.Context, id uuid.UUID, in UpdateInput) (AdminPost, error) {
	s.updateID = id
	s.updateIn = in
	if s.updateErr != nil {
		return AdminPost{}, s.updateErr
	}
	p, ok := s.posts[id]
	if !ok {
		return AdminPost{}, ErrNotFound
	}
	if in.Title != nil {
		p.Title = *in.Title
	}
	if in.Status != nil {
		p.Status = *in.Status
	}
	by := in.UpdatedBy
	p.UpdatedBy = &by
	s.posts[id] = p
	return p, nil
}

func (s *stubAdminRepo) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalledID = id
	if _, ok := s.posts[id]; !ok {
		return ErrNotFound
	}
	delete(s.posts, id)
	return nil
}

func (s *stubAdminRepo) SetPublished(_ context.Context, id uuid.UUID, publish bool, updatedBy uuid.UUID) (AdminPost, error) {
	s.publishCalls = append(s.publishCalls, struct {
		id      uuid.UUID
		publish bool
	}{id, publish})
	p, ok := s.posts[id]
	if !ok {
		return AdminPost{}, ErrNotFound
	}
	if publish {
		p.Status = "published"
		now := time.Now().UTC()
		p.PublishedAt = &now
	} else {
		p.Status = "draft"
		p.PublishedAt = nil
	}
	by := updatedBy
	p.UpdatedBy = &by
	s.posts[id] = p
	return p, nil
}

// adminTestEnv wires a router with RequireAuth + the admin posts routes against
// the provided stubs. It returns a helper to build authed requests.
type adminTestEnv struct {
	router *gin.Engine
	repo   *stubAdminRepo
	res    *resolverStub
}

func newAdminEnv(role string, perms ...string) *adminTestEnv {
	gin.SetMode(gin.TestMode)
	repo := newStubAdminRepo()
	res := &resolverStub{role: role, perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)
	return &adminTestEnv{router: r, repo: repo, res: res}
}

func (e *adminTestEnv) do(t *testing.T, uid uuid.UUID, method, path string, body any) *httptest.ResponseRecorder {
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

func TestAdmin_List_SuperAdminUnscoped(t *testing.T) {
	env := newAdminEnv("super_admin", "posts:read:all", "posts:write", "posts:publish")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodGet, "/v1/admin/posts", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Nil(t, env.repo.listFilter.CreatedBy, "elevated user must not be scoped to own posts")
}

func TestAdmin_List_AuthorScopedToOwn(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodGet, "/v1/admin/posts", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.listFilter.CreatedBy)
	assert.Equal(t, uid, *env.repo.listFilter.CreatedBy)
}

func TestAdmin_Create_AuthorPublishedForcedToDraft(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/posts", map[string]any{
		"title":   "My Post",
		"content": "body",
		"status":  "published",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "draft", env.repo.createIn.Status, "author without posts:publish must be silently downgraded")
	assert.Equal(t, "my-post", env.repo.createIn.Slug, "slug auto-generated from title")
}

func TestAdmin_Create_PublishedAllowedForEditor(t *testing.T) {
	env := newAdminEnv("editor", "posts:write", "posts:publish")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/posts", map[string]any{
		"title":   "Big News",
		"content": "body",
		"status":  "published",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "published", env.repo.createIn.Status)
}

func TestAdmin_Create_DraftDefault(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/posts", map[string]any{
		"title":   "Untitled",
		"content": "body",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "draft", env.repo.createIn.Status)
}

func TestAdmin_Create_BadInput(t *testing.T) {
	env := newAdminEnv("editor", "posts:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/posts", map[string]any{
		"title":   "", // empty
		"content": "body",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmin_Create_NoWritePerm(t *testing.T) {
	env := newAdminEnv("author") // no perms
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/posts", map[string]any{
		"title": "x", "content": "y",
	})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_Get_NotFound(t *testing.T) {
	env := newAdminEnv("super_admin", "posts:read:all", "posts:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodGet, "/v1/admin/posts/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAdmin_Patch_AuthorOwnAllowed(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	pid := uuid.New()
	env.repo.posts[pid] = AdminPost{ID: pid, Title: "T", Status: "draft", CreatedBy: &uid}

	rr := env.do(t, uid, http.MethodPatch, "/v1/admin/posts/"+pid.String(), map[string]any{
		"title": "Updated",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.updateIn.Title)
	assert.Equal(t, "Updated", *env.repo.updateIn.Title)
}

func TestAdmin_Patch_AuthorOtherForbidden(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	other := uuid.New()
	pid := uuid.New()
	env.repo.posts[pid] = AdminPost{ID: pid, Title: "T", Status: "draft", CreatedBy: &other}

	rr := env.do(t, uid, http.MethodPatch, "/v1/admin/posts/"+pid.String(), map[string]any{
		"title": "Hijack",
	})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_Patch_AuthorPublishTransitionForbidden(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	pid := uuid.New()
	env.repo.posts[pid] = AdminPost{ID: pid, Title: "T", Status: "draft", CreatedBy: &uid}

	st := "published"
	rr := env.do(t, uid, http.MethodPatch, "/v1/admin/posts/"+pid.String(), map[string]any{
		"status": st,
	})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_Delete_AuthorOwnOK(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	pid := uuid.New()
	env.repo.posts[pid] = AdminPost{ID: pid, Title: "T", Status: "draft", CreatedBy: &uid}

	rr := env.do(t, uid, http.MethodDelete, "/v1/admin/posts/"+pid.String(), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	_, exists := env.repo.posts[pid]
	assert.False(t, exists)
}

func TestAdmin_Delete_AuthorOtherForbidden(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	other := uuid.New()
	pid := uuid.New()
	env.repo.posts[pid] = AdminPost{ID: pid, Title: "T", Status: "draft", CreatedBy: &other}

	rr := env.do(t, uid, http.MethodDelete, "/v1/admin/posts/"+pid.String(), nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_Publish_OK(t *testing.T) {
	env := newAdminEnv("editor", "posts:write", "posts:publish")
	uid := uuid.New()
	pid := uuid.New()
	env.repo.posts[pid] = AdminPost{ID: pid, Title: "T", Status: "draft", CreatedBy: &uid}

	rr := env.do(t, uid, http.MethodPost, "/v1/admin/posts/"+pid.String()+"/publish", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Len(t, env.repo.publishCalls, 1)
	assert.True(t, env.repo.publishCalls[0].publish)
}

func TestAdmin_Unpublish_RequiresPublishPerm(t *testing.T) {
	env := newAdminEnv("author", "posts:write")
	uid := uuid.New()
	pid := uuid.New()
	env.repo.posts[pid] = AdminPost{ID: pid, Title: "T", Status: "published", CreatedBy: &uid}

	rr := env.do(t, uid, http.MethodPost, "/v1/admin/posts/"+pid.String()+"/unpublish", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_Get_MissingAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newStubAdminRepo()
	res := &resolverStub{role: "editor", perms: map[string]bool{}}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/posts", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
