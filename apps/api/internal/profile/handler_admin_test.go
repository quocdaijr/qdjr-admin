package profile

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

type stubAdminRepo struct {
	current  Profile
	updateIn UpdateInput
	updated  bool
	getErr   error
	updErr   error
}

func (s *stubAdminRepo) Get(_ context.Context) (Profile, error) {
	if s.getErr != nil {
		return Profile{}, s.getErr
	}
	return s.current, nil
}
func (s *stubAdminRepo) Update(_ context.Context, in UpdateInput) (Profile, error) {
	s.updateIn = in
	s.updated = true
	if s.updErr != nil {
		return Profile{}, s.updErr
	}
	if in.FullName != nil {
		s.current.FullName = in.FullName
	}
	if in.Email != nil {
		s.current.Email = in.Email
	}
	if in.SocialLinks != nil {
		m := *in.SocialLinks
		cp := make(map[string]string, len(m))
		for k, v := range m {
			cp[k] = v
		}
		s.current.SocialLinks = cp
	}
	return s.current, nil
}

type adminEnv struct {
	router *gin.Engine
	repo   *stubAdminRepo
	res    *resolverStub
}

func newAdminEnv(perms ...string) *adminEnv {
	gin.SetMode(gin.TestMode)
	repo := &stubAdminRepo{current: Profile{ID: 1, SocialLinks: map[string]string{}, UpdatedAt: time.Now().UTC()}}
	res := &resolverStub{role: "admin", perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)
	return &adminEnv{router: r, repo: repo, res: res}
}

func (e *adminEnv) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
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
		"sub": uuid.New().String(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, _ := tok.SignedString([]byte(adminTestSecret))
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func (e *adminEnv) doRaw(t *testing.T, method, path, raw string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uuid.New().String(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, _ := tok.SignedString([]byte(adminTestSecret))
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func TestAdminProfile_Get_OK(t *testing.T) {
	env := newAdminEnv("profile:write")
	email := "me@example.com"
	env.repo.current.Email = &email
	rr := env.do(t, http.MethodGet, "/v1/admin/profile", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Data Profile `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Data.Email)
	assert.Equal(t, "me@example.com", *body.Data.Email)
}

func TestAdminProfile_Get_Forbidden(t *testing.T) {
	env := newAdminEnv()
	rr := env.do(t, http.MethodGet, "/v1/admin/profile", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminProfile_Patch_PartialFields(t *testing.T) {
	env := newAdminEnv("profile:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/profile", map[string]any{
		"full_name": "New Name",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.True(t, env.repo.updated)
	require.NotNil(t, env.repo.updateIn.FullName)
	assert.Equal(t, "New Name", *env.repo.updateIn.FullName)
	// Other fields untouched.
	assert.Nil(t, env.repo.updateIn.Bio)
	assert.Nil(t, env.repo.updateIn.SocialLinks)
}

func TestAdminProfile_Patch_SocialLinksClear(t *testing.T) {
	env := newAdminEnv("profile:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/profile", map[string]any{
		"social_links": map[string]string{},
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.updateIn.SocialLinks)
	assert.Empty(t, *env.repo.updateIn.SocialLinks)
}

func TestAdminProfile_Patch_AvatarIDExplicitNull(t *testing.T) {
	env := newAdminEnv("profile:write")
	rr := env.doRaw(t, http.MethodPatch, "/v1/admin/profile", `{"avatar_id": null}`)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.updateIn.AvatarID, "avatar_id key was present, should be set")
	assert.Nil(t, *env.repo.updateIn.AvatarID, "value is null → clear avatar")
}

func TestAdminProfile_Patch_AvatarIDValue(t *testing.T) {
	env := newAdminEnv("profile:write")
	id := uuid.New()
	rr := env.doRaw(t, http.MethodPatch, "/v1/admin/profile", `{"avatar_id":"`+id.String()+`"}`)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.updateIn.AvatarID)
	require.NotNil(t, *env.repo.updateIn.AvatarID)
	assert.Equal(t, id, **env.repo.updateIn.AvatarID)
}

func TestAdminProfile_Patch_AvatarIDMissingLeavesUntouched(t *testing.T) {
	env := newAdminEnv("profile:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/profile", map[string]any{
		"full_name": "x",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Nil(t, env.repo.updateIn.AvatarID, "avatar_id absent → AvatarID nil")
}

func TestAdminProfile_Patch_BadAvatarUUID(t *testing.T) {
	env := newAdminEnv("profile:write")
	rr := env.doRaw(t, http.MethodPatch, "/v1/admin/profile", `{"avatar_id":"not-a-uuid"}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminProfile_Patch_Forbidden(t *testing.T) {
	env := newAdminEnv()
	rr := env.do(t, http.MethodPatch, "/v1/admin/profile", map[string]any{"full_name": "x"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminProfile_MissingAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &stubAdminRepo{}
	res := &resolverStub{role: "admin", perms: map[string]bool{"profile:write": true}}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/profile", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
