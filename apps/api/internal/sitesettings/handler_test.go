package sitesettings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubReader struct {
	out Public
	err error
}

func (s *stubReader) GetPublic(_ context.Context) (Public, error) {
	return s.out, s.err
}

func newRouter(reader Reader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/v1")
	RegisterPublic(g, reader)
	return r
}

func TestGetPublic_OK(t *testing.T) {
	desc := "personal site"
	footer := "(c) 2026"
	reader := &stubReader{out: Public{
		SiteTitle:       "qdjr.me",
		SiteDescription: &desc,
		FooterText:      &footer,
		SocialLinks:     map[string]string{"github": "quocdaijr"},
	}}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/site-settings", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data  Public `json:"data"`
		Error any    `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Error)
	assert.Equal(t, "qdjr.me", body.Data.SiteTitle)
	require.NotNil(t, body.Data.SiteDescription)
	assert.Equal(t, "personal site", *body.Data.SiteDescription)
	assert.Equal(t, "quocdaijr", body.Data.SocialLinks["github"])

	// Sanity: the raw response must NOT contain contact_email key.
	assert.NotContains(t, rr.Body.String(), "contact_email")
}

func TestGetPublic_RepoError(t *testing.T) {
	reader := &stubReader{err: errString("boom")}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/site-settings", nil))
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

type errString string

func (e errString) Error() string { return string(e) }
