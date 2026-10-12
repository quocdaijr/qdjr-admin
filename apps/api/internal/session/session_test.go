package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/contact"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/users"
)

const secret = "session-test-secret-session-test-secret"

type memStore struct {
	byEmail map[string]users.Credential
	touched []uuid.UUID
}

func (m *memStore) FindByEmail(_ context.Context, email string) (users.Credential, error) {
	if c, ok := m.byEmail[email]; ok {
		return c, nil
	}
	return users.Credential{}, users.ErrNotFound
}

func (m *memStore) FindByID(_ context.Context, id uuid.UUID) (users.Credential, error) {
	for _, c := range m.byEmail {
		if c.ID == id {
			return c, nil
		}
	}
	return users.Credential{}, users.ErrNotFound
}

func (m *memStore) TouchSignIn(_ context.Context, id uuid.UUID) error {
	m.touched = append(m.touched, id)
	return nil
}

func (m *memStore) SetPassword(_ context.Context, id uuid.UUID, hash string) error {
	for k, c := range m.byEmail {
		if c.ID == id {
			m.byEmail[k] = users.Credential{ID: c.ID, Email: c.Email, PasswordHash: hash}
			return nil
		}
	}
	return users.ErrNotFound
}

type env struct {
	r     *gin.Engine
	store *memStore
	iss   *auth.Issuer
	id    uuid.UUID
}

func newEnv(t *testing.T, burst int) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hash, err := auth.HashPassword("correct-password")
	require.NoError(t, err)
	id := uuid.New()
	store := &memStore{byEmail: map[string]users.Credential{
		"admin@qdjr.dev": {ID: id, Email: "admin@qdjr.dev", PasswordHash: hash},
	}}
	iss := auth.NewIssuer([]byte(secret), 15*time.Minute, time.Hour)
	r := gin.New()
	RegisterPublic(r.Group("/v1"), store, iss, contact.NewLimiterWith(rate.Every(time.Hour), burst))
	RegisterAdmin(r.Group("/v1/admin", apphttp.RequireAuth(iss)), store)
	return &env{r: r, store: store, iss: iss, id: id}
}

func (e *env) do(method, path, token string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.7:1234"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	e.r.ServeHTTP(rr, req)
	return rr
}

type loginBody struct {
	Data struct {
		auth.TokenPair
		User struct {
			ID    uuid.UUID `json:"id"`
			Email string    `json:"email"`
		} `json:"user"`
	} `json:"data"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func TestLogin_OK(t *testing.T) {
	e := newEnv(t, 10)
	rr := e.do(http.MethodPost, "/v1/auth/login", "", map[string]string{"email": " Admin@QDJR.dev ", "password": "correct-password"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var b loginBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &b))
	assert.Equal(t, e.id, b.Data.User.ID)
	assert.Equal(t, "admin@qdjr.dev", b.Data.User.Email)
	claims, err := e.iss.Verify(b.Data.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, e.id, claims.UserID)
	assert.Equal(t, []uuid.UUID{e.id}, e.store.touched)
}

func TestLogin_WrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	e := newEnv(t, 10)
	a := e.do(http.MethodPost, "/v1/auth/login", "", map[string]string{"email": "admin@qdjr.dev", "password": "wrong-password"})
	b := e.do(http.MethodPost, "/v1/auth/login", "", map[string]string{"email": "nobody@qdjr.dev", "password": "wrong-password"})
	assert.Equal(t, http.StatusUnauthorized, a.Code)
	assert.Equal(t, http.StatusUnauthorized, b.Code)
	assert.JSONEq(t, a.Body.String(), b.Body.String())
	assert.Empty(t, e.store.touched)
}

func TestLogin_RateLimited(t *testing.T) {
	e := newEnv(t, 2)
	for i := 0; i < 2; i++ {
		e.do(http.MethodPost, "/v1/auth/login", "", map[string]string{"email": "admin@qdjr.dev", "password": "wrong-password"})
	}
	rr := e.do(http.MethodPost, "/v1/auth/login", "", map[string]string{"email": "admin@qdjr.dev", "password": "correct-password"})
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRefresh_IssuesNewPairAndRejectsAccessTokenOrDeletedUser(t *testing.T) {
	e := newEnv(t, 10)
	pair, err := e.iss.Issue(e.id, "admin@qdjr.dev")
	require.NoError(t, err)

	rr := e.do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": pair.RefreshToken})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = e.do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": pair.AccessToken})
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "access token cannot refresh")

	delete(e.store.byEmail, "admin@qdjr.dev")
	rr = e.do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": pair.RefreshToken})
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "deleted user cannot refresh")
}

func TestRefreshTokenIsNotAnAccessToken(t *testing.T) {
	e := newEnv(t, 10)
	pair, err := e.iss.Issue(e.id, "admin@qdjr.dev")
	require.NoError(t, err)
	rr := e.do(http.MethodPatch, "/v1/admin/me/password", pair.RefreshToken, map[string]string{})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestChangePassword(t *testing.T) {
	e := newEnv(t, 10)
	pair, err := e.iss.Issue(e.id, "admin@qdjr.dev")
	require.NoError(t, err)

	rr := e.do(http.MethodPatch, "/v1/admin/me/password", pair.AccessToken,
		map[string]string{"current_password": "wrong-password", "new_password": "brand-new-password"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "WRONG_PASSWORD")

	rr = e.do(http.MethodPatch, "/v1/admin/me/password", pair.AccessToken,
		map[string]string{"current_password": "correct-password", "new_password": "short"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = e.do(http.MethodPatch, "/v1/admin/me/password", pair.AccessToken,
		map[string]string{"current_password": "correct-password", "new_password": "brand-new-password"})
	require.Equal(t, http.StatusNoContent, rr.Code, rr.Body.String())
	assert.True(t, auth.CheckPassword(e.store.byEmail["admin@qdjr.dev"].PasswordHash, "brand-new-password"))
}
