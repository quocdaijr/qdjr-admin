package media

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

const adminTestSecret = "test-secret-test-secret-test-secret"

// stubRepo is an in-memory AdminWriter for handler tests.
type stubRepo struct {
	rows         map[uuid.UUID]Media
	createIn     CreateInput
	createErr    error
	deleteCalled uuid.UUID
	getErr       error
}

func newStubRepo() *stubRepo {
	return &stubRepo{rows: map[uuid.UUID]Media{}}
}

func (s *stubRepo) List(_ context.Context, f AdminListFilter) ([]Media, int, error) {
	out := make([]Media, 0, len(s.rows))
	for _, m := range s.rows {
		out = append(out, m)
	}
	return out, len(out), nil
}

func (s *stubRepo) Get(_ context.Context, id uuid.UUID) (Media, error) {
	if s.getErr != nil {
		return Media{}, s.getErr
	}
	m, ok := s.rows[id]
	if !ok {
		return Media{}, ErrNotFound
	}
	return m, nil
}

func (s *stubRepo) Create(_ context.Context, in CreateInput) (Media, error) {
	s.createIn = in
	if s.createErr != nil {
		return Media{}, s.createErr
	}
	id := uuid.New()
	by := in.UploadedBy
	m := Media{
		ID:          id,
		Filename:    in.Filename,
		StoragePath: in.StoragePath,
		MimeType:    in.MimeType,
		Size:        in.Size,
		Width:       in.Width,
		Height:      in.Height,
		AltText:     in.AltText,
		UploadedBy:  &by,
		CreatedAt:   time.Now().UTC(),
	}
	s.rows[id] = m
	return m, nil
}

func (s *stubRepo) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = id
	if _, ok := s.rows[id]; !ok {
		return ErrNotFound
	}
	delete(s.rows, id)
	return nil
}

// stubStorage is an in-memory Storage stub for handler tests.
type stubStorage struct {
	signedURL    string
	signedErr    error
	signedPath   string
	deleted      []string
	deleteErr    error
}

func (s *stubStorage) SignedUploadURL(_ context.Context, path string) (string, error) {
	s.signedPath = path
	if s.signedErr != nil {
		return "", s.signedErr
	}
	if s.signedURL != "" {
		return s.signedURL, nil
	}
	return "https://stor.example/upload/" + path + "?token=t", nil
}

func (s *stubStorage) Delete(_ context.Context, path string) error {
	s.deleted = append(s.deleted, path)
	return s.deleteErr
}

// resolverStub is a minimal PermissionResolver for handler tests.
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

type adminTestEnv struct {
	router  *gin.Engine
	repo    *stubRepo
	storage *stubStorage
	res     *resolverStub
}

func newAdminEnv(role string, perms ...string) *adminTestEnv {
	gin.SetMode(gin.TestMode)
	repo := newStubRepo()
	storage := &stubStorage{}
	res := &resolverStub{role: role, perms: map[string]bool{}}
	for _, p := range perms {
		res.perms[p] = true
	}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, storage, res)
	return &adminTestEnv{router: r, repo: repo, storage: storage, res: res}
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

func TestAdmin_List_OK(t *testing.T) {
	env := newAdminEnv("editor", "media:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodGet, "/v1/admin/media", nil)
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestAdmin_List_Forbidden(t *testing.T) {
	env := newAdminEnv("author") // no perms
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodGet, "/v1/admin/media", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_SignedURL_OK(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media/signed-url", map[string]any{
		"filename":  "logo.png",
		"mime_type": "image/png",
		"size":      2048,
	})
	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Data signedURLResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.True(t, len(body.Data.StoragePath) > len("media/"), "storage_path should be set")
	assert.NotEmpty(t, body.Data.SignedURL)
	assert.Equal(t, signedURLTTL, body.Data.ExpiresIn)
	// Storage was called with the matching path.
	assert.Equal(t, body.Data.StoragePath, env.storage.signedPath)
}

func TestAdmin_SignedURL_BadMime(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media/signed-url", map[string]any{
		"filename":  "doc.pdf",
		"mime_type": "application/pdf",
		"size":      2048,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmin_SignedURL_OversizedRejected(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media/signed-url", map[string]any{
		"filename":  "big.png",
		"mime_type": "image/png",
		"size":      MaxUploadSize + 1,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmin_SignedURL_StorageFailure(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	env.storage.signedErr = errors.New("boom")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media/signed-url", map[string]any{
		"filename":  "ok.png",
		"mime_type": "image/png",
		"size":      512,
	})
	assert.Equal(t, http.StatusBadGateway, rr.Code)
}

func TestAdmin_Create_OK(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	path := "media/" + uuid.New().String() + ".png"
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media", map[string]any{
		"filename":     "ok.png",
		"storage_path": path,
		"mime_type":    "image/png",
		"size":         1024,
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, path, env.repo.createIn.StoragePath)
	assert.Equal(t, uid, env.repo.createIn.UploadedBy)
}

func TestAdmin_Create_BadStoragePath(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media", map[string]any{
		"filename":     "ok.png",
		"storage_path": "wrong/path.png",
		"mime_type":    "image/png",
		"size":         1024,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmin_Create_AltTextTooLong(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	long := make([]byte, 501)
	for i := range long {
		long[i] = 'a'
	}
	alt := string(long)
	path := "media/" + uuid.New().String() + ".png"
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media", map[string]any{
		"filename":     "ok.png",
		"storage_path": path,
		"mime_type":    "image/png",
		"size":         1024,
		"alt_text":     alt,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmin_Create_NoWritePerm(t *testing.T) {
	env := newAdminEnv("author") // no media:write
	uid := uuid.New()
	path := "media/" + uuid.New().String() + ".png"
	rr := env.do(t, uid, http.MethodPost, "/v1/admin/media", map[string]any{
		"filename":     "x.png",
		"storage_path": path,
		"mime_type":    "image/png",
		"size":         1024,
	})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_Delete_AuthorOwnOK(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	id := uuid.New()
	by := uid
	env.repo.rows[id] = Media{
		ID:          id,
		StoragePath: "media/" + uuid.New().String() + ".png",
		UploadedBy:  &by,
	}
	rr := env.do(t, uid, http.MethodDelete, "/v1/admin/media/"+id.String(), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	_, exists := env.repo.rows[id]
	assert.False(t, exists)
	require.Len(t, env.storage.deleted, 1)
}

func TestAdmin_Delete_AuthorOtherForbidden(t *testing.T) {
	env := newAdminEnv("author", "media:write")
	uid := uuid.New()
	other := uuid.New()
	id := uuid.New()
	env.repo.rows[id] = Media{
		ID:          id,
		StoragePath: "media/" + uuid.New().String() + ".png",
		UploadedBy:  &other,
	}
	rr := env.do(t, uid, http.MethodDelete, "/v1/admin/media/"+id.String(), nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdmin_Delete_EditorBypassesOwnership(t *testing.T) {
	env := newAdminEnv("editor", "media:write", "posts:read:all")
	uid := uuid.New()
	other := uuid.New()
	id := uuid.New()
	env.repo.rows[id] = Media{
		ID:          id,
		StoragePath: "media/" + uuid.New().String() + ".png",
		UploadedBy:  &other,
	}
	rr := env.do(t, uid, http.MethodDelete, "/v1/admin/media/"+id.String(), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdmin_Delete_NotFound(t *testing.T) {
	env := newAdminEnv("editor", "media:write", "posts:read:all")
	uid := uuid.New()
	rr := env.do(t, uid, http.MethodDelete, "/v1/admin/media/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAdmin_Delete_StorageFailure(t *testing.T) {
	env := newAdminEnv("editor", "media:write", "posts:read:all")
	env.storage.deleteErr = errors.New("net down")
	uid := uuid.New()
	id := uuid.New()
	by := uid
	env.repo.rows[id] = Media{
		ID:          id,
		StoragePath: "media/" + uuid.New().String() + ".png",
		UploadedBy:  &by,
	}
	rr := env.do(t, uid, http.MethodDelete, "/v1/admin/media/"+id.String(), nil)
	assert.Equal(t, http.StatusBadGateway, rr.Code)
	// DB row must remain because storage delete failed.
	_, exists := env.repo.rows[id]
	assert.True(t, exists)
}

func TestAdmin_MissingAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newStubRepo()
	storage := &stubStorage{}
	res := &resolverStub{role: "editor", perms: map[string]bool{"media:write": true}}
	v := auth.NewHS256Verifier([]byte(adminTestSecret))
	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	RegisterAdmin(g, repo, storage, res)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/media", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
