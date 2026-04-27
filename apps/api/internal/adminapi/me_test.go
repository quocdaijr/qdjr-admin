package adminapi

import (
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

const testSecret = "test-secret-test-secret-test-secret"

type stubResolver struct {
	role  string
	perms []string
}

func (s *stubResolver) Role(_ context.Context, _ uuid.UUID) (string, error) {
	return s.role, nil
}
func (s *stubResolver) Permissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	return s.perms, nil
}
func (s *stubResolver) Can(_ context.Context, _ uuid.UUID, p string) (bool, error) {
	for _, x := range s.perms {
		if x == p {
			return true, nil
		}
	}
	return false, nil
}

func TestMe_ReturnsUserAndRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uid := uuid.New()
	res := &stubResolver{role: "editor", perms: []string{"posts:write"}}
	v := auth.NewHS256Verifier([]byte(testSecret))

	r := gin.New()
	g := r.Group("/v1/admin", apphttp.RequireAuth(v))
	Register(g, Deps{Resolver: res})

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   uid.String(),
		"email": "alice@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	signed, _ := tok.SignedString([]byte(testSecret))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Data struct {
			UserID      string   `json:"user_id"`
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, uid.String(), body.Data.UserID)
	assert.Equal(t, "editor", body.Data.Role)
	assert.Contains(t, body.Data.Permissions, "posts:write")
}
