package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

func (s *stubRepo) List(_ context.Context, _ AdminListFilter) ([]Media, int, error) {
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
	put       map[string][]byte
	putType   map[string]string
	putErr    error
	deleted   []string
	deleteErr error
}

func (s *stubStorage) Put(_ context.Context, key, contentType string, body io.Reader, _ int64) error {
	if s.putErr != nil {
		return s.putErr
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if s.put == nil {
		s.put, s.putType = map[string][]byte{}, map[string]string{}
	}
	s.put[key], s.putType[key] = b, contentType
	return nil
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

func bearer(uid uuid.UUID) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uid.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, _ := tok.SignedString([]byte(adminTestSecret))
	return "Bearer " + signed
}

// upload posts a multipart form with one "file" part (and optional alt_text).
func (e *adminTestEnv) upload(t *testing.T, uid uuid.UUID, filename string, content []byte, altText string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	if altText != "" {
		require.NoError(t, w.WriteField("alt_text", altText))
	}
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/media", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", bearer(uid))
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

// pngBytes returns a real w×h PNG.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))))
	return buf.Bytes()
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

func TestAdmin_Upload_PNG_OK(t *testing.T) {
	env := newAdminEnv("editor", "media:write")
	uid := uuid.New()
	rr := env.upload(t, uid, "photo.PNG", pngBytes(t, 3, 2), "a tiny test")
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	in := env.repo.createIn
	assert.Regexp(t, `^media/[0-9a-f-]{36}\.png$`, in.StoragePath)
	assert.Equal(t, "image/png", in.MimeType, "type comes from the bytes")
	assert.Equal(t, "photo.PNG", in.Filename)
	require.NotNil(t, in.Width)
	require.NotNil(t, in.Height)
	assert.Equal(t, 3, *in.Width)
	assert.Equal(t, 2, *in.Height)
	require.NotNil(t, in.AltText)
	assert.Equal(t, "a tiny test", *in.AltText)
	assert.Equal(t, uid, in.UploadedBy)
	assert.Equal(t, "image/png", env.storage.putType[in.StoragePath])
	assert.Equal(t, int64(len(env.storage.put[in.StoragePath])), in.Size)
}

func TestAdmin_Upload_RejectsSVGAndText(t *testing.T) {
	cases := map[string][]byte{
		"evil.svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"note.png":  []byte("just text pretending to be a png"),
		"empty.png": {},
	}
	for name, body := range cases {
		env := newAdminEnv("editor", "media:write")
		rr := env.upload(t, uuid.New(), name, body, "")
		assert.Equal(t, http.StatusBadRequest, rr.Code, name)
		assert.Empty(t, env.storage.put, name)
	}
}

func TestAdmin_Upload_TooLarge(t *testing.T) {
	env := newAdminEnv("editor", "media:write")
	big := append(pngBytes(t, 1, 1), bytes.Repeat([]byte{0}, int(MaxUploadSize))...)
	rr := env.upload(t, uuid.New(), "big.png", big, "")
	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Empty(t, env.storage.put)
}

func TestAdmin_Upload_MissingFile(t *testing.T) {
	env := newAdminEnv("editor", "media:write")
	rr := env.do(t, uuid.New(), http.MethodPost, "/v1/admin/media", map[string]any{"filename": "x.png"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdmin_Upload_StorageFailureCreatesNoRow(t *testing.T) {
	env := newAdminEnv("editor", "media:write")
	env.storage.putErr = errors.New("r2 down")
	rr := env.upload(t, uuid.New(), "a.png", pngBytes(t, 1, 1), "")
	assert.Equal(t, http.StatusBadGateway, rr.Code)
	assert.Empty(t, env.repo.rows)
}

func TestAdmin_Upload_DBFailureRemovesStoredFile(t *testing.T) {
	env := newAdminEnv("editor", "media:write")
	env.repo.createErr = errors.New("db down")
	rr := env.upload(t, uuid.New(), "a.png", pngBytes(t, 1, 1), "")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Len(t, env.storage.put, 1)
	for key := range env.storage.put {
		assert.Equal(t, []string{key}, env.storage.deleted, "orphaned object is cleaned up")
	}
}

func TestAdmin_Upload_AltTextTooLong(t *testing.T) {
	env := newAdminEnv("editor", "media:write")
	rr := env.upload(t, uuid.New(), "a.png", pngBytes(t, 1, 1), strings.Repeat("x", 501))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Empty(t, env.storage.put)
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
