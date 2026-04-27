package contact

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
	items     map[uuid.UUID]Message
	listFilt  AdminListFilter
	updateID  uuid.UUID
	updateSt  string
	updateErr error
}

func (s *stubAdminRepo) List(_ context.Context, f AdminListFilter) ([]Message, int, error) {
	s.listFilt = f
	out := make([]Message, 0, len(s.items))
	for _, m := range s.items {
		if f.Status == "" || m.Status == f.Status {
			out = append(out, m)
		}
	}
	return out, len(out), nil
}

func (s *stubAdminRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) (Message, error) {
	s.updateID = id
	s.updateSt = status
	if s.updateErr != nil {
		return Message{}, s.updateErr
	}
	m, ok := s.items[id]
	if !ok {
		return Message{}, ErrNotFound
	}
	m.Status = status
	s.items[id] = m
	return m, nil
}

type adminEnv struct {
	router *gin.Engine
	repo   *stubAdminRepo
}

func newAdminEnv(perms ...string) *adminEnv {
	gin.SetMode(gin.TestMode)
	repo := &stubAdminRepo{items: map[uuid.UUID]Message{}}
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

func TestAdminContact_List_OK(t *testing.T) {
	env := newAdminEnv("contact:read")
	rr := env.do(t, http.MethodGet, "/v1/admin/contact-messages?page=2&perPage=5&status=new", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 2, env.repo.listFilt.Page)
	assert.Equal(t, 5, env.repo.listFilt.PerPage)
	assert.Equal(t, "new", env.repo.listFilt.Status)
}

func TestAdminContact_List_PerPageClamped(t *testing.T) {
	env := newAdminEnv("contact:read")
	rr := env.do(t, http.MethodGet, "/v1/admin/contact-messages?perPage=500", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 100, env.repo.listFilt.PerPage)
}

func TestAdminContact_List_BadStatus(t *testing.T) {
	env := newAdminEnv("contact:read")
	rr := env.do(t, http.MethodGet, "/v1/admin/contact-messages?status=bogus", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminContact_List_Forbidden(t *testing.T) {
	env := newAdminEnv()
	rr := env.do(t, http.MethodGet, "/v1/admin/contact-messages", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminContact_Patch_OK(t *testing.T) {
	env := newAdminEnv("contact:write")
	id := uuid.New()
	env.repo.items[id] = Message{ID: id, Name: "A", Email: "a@example.com", Body: "hi", Status: "new"}

	rr := env.do(t, http.MethodPatch, "/v1/admin/contact-messages/"+id.String(), map[string]any{
		"status": "read",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, id, env.repo.updateID)
	assert.Equal(t, "read", env.repo.updateSt)
}

func TestAdminContact_Patch_BadStatus(t *testing.T) {
	env := newAdminEnv("contact:write")
	id := uuid.New()
	env.repo.items[id] = Message{ID: id, Status: "new"}
	rr := env.do(t, http.MethodPatch, "/v1/admin/contact-messages/"+id.String(), map[string]any{
		"status": "bogus",
	})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "VALIDATION", body.Error.Code)
}

func TestAdminContact_Patch_BadID(t *testing.T) {
	env := newAdminEnv("contact:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/contact-messages/not-a-uuid", map[string]any{"status": "read"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminContact_Patch_NotFound(t *testing.T) {
	env := newAdminEnv("contact:write")
	rr := env.do(t, http.MethodPatch, "/v1/admin/contact-messages/"+uuid.New().String(), map[string]any{"status": "read"})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAdminContact_Patch_Forbidden(t *testing.T) {
	env := newAdminEnv() // no perms
	rr := env.do(t, http.MethodPatch, "/v1/admin/contact-messages/"+uuid.New().String(), map[string]any{"status": "read"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminContact_MissingAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &stubAdminRepo{items: map[uuid.UUID]Message{}}
	res := &resolverStub{role: "admin", perms: map[string]bool{"contact:read": true}}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, res)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/contact-messages", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
