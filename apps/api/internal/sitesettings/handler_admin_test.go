package sitesettings

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
	current  Admin
	updateIn UpdateInput
}

func (s *stubAdminRepo) Get(_ context.Context) (Admin, error) { return s.current, nil }
func (s *stubAdminRepo) Update(_ context.Context, in UpdateInput) (Admin, error) {
	s.updateIn = in
	if in.SiteTitle != nil {
		s.current.SiteTitle = *in.SiteTitle
	}
	if in.ContactEmail != nil {
		s.current.ContactEmail = in.ContactEmail
	}
	return s.current, nil
}

type adminEnv struct {
	router *gin.Engine
	repo   *stubAdminRepo
}

func newAdminEnv(perms ...string) *adminEnv {
	gin.SetMode(gin.TestMode)
	repo := &stubAdminRepo{current: Admin{ID: 1, SiteTitle: "qdjr.me", SocialLinks: map[string]string{}, UpdatedAt: time.Now().UTC()}}
	res := &resolverStub{role: "admin", perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)
	return &adminEnv{router: r, repo: repo}
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

func TestAdminSiteSettings_Get_IncludesContactEmail(t *testing.T) {
	env := newAdminEnv("settings:write")
	email := "admin@example.com"
	env.repo.current.ContactEmail = &email
	rr := env.do(t, http.MethodGet, "/v1/admin/site-settings", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data Admin `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Data.ContactEmail)
	assert.Equal(t, "admin@example.com", *body.Data.ContactEmail)
}

func TestAdminSiteSettings_Get_Forbidden(t *testing.T) {
	env := newAdminEnv()
	rr := env.do(t, http.MethodGet, "/v1/admin/site-settings", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminSiteSettings_Patch_OK(t *testing.T) {
	env := newAdminEnv("settings:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/site-settings", map[string]any{
		"site_title":    "New Title",
		"contact_email": "new@example.com",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.updateIn.SiteTitle)
	assert.Equal(t, "New Title", *env.repo.updateIn.SiteTitle)
	require.NotNil(t, env.repo.updateIn.ContactEmail)
	assert.Equal(t, "new@example.com", *env.repo.updateIn.ContactEmail)
}

func TestAdminSiteSettings_Patch_BadTitle(t *testing.T) {
	env := newAdminEnv("settings:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/site-settings", map[string]any{
		"site_title": "",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminSiteSettings_Patch_SocialLinksEmptyClears(t *testing.T) {
	env := newAdminEnv("settings:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/site-settings", map[string]any{
		"social_links": map[string]string{},
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, env.repo.updateIn.SocialLinks)
	assert.Empty(t, *env.repo.updateIn.SocialLinks)
}

func TestAdminSiteSettings_Patch_Forbidden(t *testing.T) {
	env := newAdminEnv()
	rr := env.do(t, http.MethodPatch, "/v1/admin/site-settings", map[string]any{"site_title": "x"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminSiteSettings_MissingAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &stubAdminRepo{}
	res := &resolverStub{role: "admin", perms: map[string]bool{"settings:write": true}}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/site-settings", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
