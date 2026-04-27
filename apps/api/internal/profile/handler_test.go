package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubReader is a Reader test double.
type stubReader struct {
	out Profile
	err error
}

func (s *stubReader) Get(_ context.Context) (Profile, error) {
	return s.out, s.err
}

func newRouter(reader Reader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/v1")
	RegisterPublic(g, reader)
	return r
}

func TestGet_OK(t *testing.T) {
	now := time.Now().UTC()
	name := "Quoc Dai"
	url := "https://cdn.example/avatar.jpg"
	reader := &stubReader{out: Profile{
		ID:          1,
		FullName:    &name,
		AvatarURL:   &url,
		SocialLinks: map[string]string{"github": "quocdaijr"},
		UpdatedAt:   now,
	}}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/profile", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data  Profile `json:"data"`
		Error any     `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Error)
	assert.Equal(t, int16(1), body.Data.ID)
	require.NotNil(t, body.Data.FullName)
	assert.Equal(t, "Quoc Dai", *body.Data.FullName)
	require.NotNil(t, body.Data.AvatarURL)
	assert.Equal(t, url, *body.Data.AvatarURL)
	assert.Equal(t, "quocdaijr", body.Data.SocialLinks["github"])
}

func TestGet_RepoError(t *testing.T) {
	reader := &stubReader{err: assertErr("boom")}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/profile", nil))
	require.Equal(t, http.StatusInternalServerError, rr.Code)

	var body struct {
		Data  any `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL", body.Error.Code)
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
