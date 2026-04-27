package http

import (
	"context"
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
)

const testSecret = "test-secret-test-secret-test-secret"

func newTestJWT(t *testing.T, sub uuid.UUID) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub.String(),
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

type stubResolver struct {
	role  string
	perms []string
	err   error
}

func (s *stubResolver) Role(_ context.Context, _ uuid.UUID) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.role, nil
}
func (s *stubResolver) Permissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	return s.perms, s.err
}
func (s *stubResolver) Can(_ context.Context, _ uuid.UUID, perm string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	for _, p := range s.perms {
		if p == perm {
			return true, nil
		}
	}
	return false, nil
}

func TestRequireAuth_RejectsMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v := auth.NewHS256Verifier([]byte(testSecret))
	r.GET("/x", RequireAuth(v), func(c *gin.Context) { c.Status(http.StatusOK) })

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAuth_AttachesUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uid := uuid.New()
	r := gin.New()
	v := auth.NewHS256Verifier([]byte(testSecret))
	r.GET("/x", RequireAuth(v), func(c *gin.Context) {
		got, ok := UserIDFromContext(c)
		require.True(t, ok)
		assert.Equal(t, uid, got)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+newTestJWT(t, uid))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequirePermission_AllowsAndDenies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uid := uuid.New()
	v := auth.NewHS256Verifier([]byte(testSecret))

	cases := []struct {
		name    string
		perms   []string
		wantSts int
	}{
		{"allowed", []string{"posts:write"}, http.StatusOK},
		{"denied", []string{}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := &stubResolver{role: "author", perms: tc.perms}
			r := gin.New()
			r.GET("/x", RequireAuth(v), RequirePermission(res, "posts:write"), func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set("Authorization", "Bearer "+newTestJWT(t, uid))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, tc.wantSts, rr.Code)
		})
	}
}
