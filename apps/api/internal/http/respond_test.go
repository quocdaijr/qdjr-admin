package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOK_WrapsDataAndNullsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) { OK(c, map[string]int{"n": 7}) })

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Data  map[string]int `json:"data"`
		Error any            `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, 7, body.Data["n"])
	assert.Nil(t, body.Error)
}

func TestList_IncludesMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) { List(c, []int{1, 2}, Meta{Page: 1, PerPage: 10, Total: 2}) })

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Data []int `json:"data"`
		Meta Meta  `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, []int{1, 2}, body.Data)
	assert.Equal(t, 2, body.Meta.Total)
}

func TestErr_SetsStatusAndCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) { Err(c, http.StatusForbidden, "FORBIDDEN", "no") })

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.Equal(t, http.StatusForbidden, rr.Code)
	var body struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Data)
	assert.Equal(t, "FORBIDDEN", body.Error.Code)
}
