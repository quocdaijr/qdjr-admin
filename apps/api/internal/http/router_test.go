package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

func TestRouter_HealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(RouterDeps{Pool: (*pgxpool.Pool)(nil), CORSOrigins: []string{"http://localhost:3000"}})

	for _, path := range []string{"/healthz", "/readyz"} {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusOK, rr.Code, "GET %s", path)
	}
}

func TestRouter_CORS_PreflightAllowsListedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(RouterDeps{CORSOrigins: []string{"http://localhost:3000"}})

	req := httptest.NewRequest(http.MethodOptions, "/v1/posts", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
}
