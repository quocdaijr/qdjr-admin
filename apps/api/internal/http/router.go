package http

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RouterDeps wires runtime dependencies into the HTTP layer. New deps land here
// rather than as global state.
type RouterDeps struct {
	Pool        *pgxpool.Pool
	CORSOrigins []string
}

// NewRouter builds the Gin engine with the standard middleware stack and
// registers /healthz and /readyz. Resource routes are added by Plan 2.
func NewRouter(deps RouterDeps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), corsMiddleware(deps.CORSOrigins))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		if deps.Pool == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "skipped"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		if err := deps.Pool.Ping(ctx); err != nil {
			Err(c, http.StatusServiceUnavailable, "DB_UNREACHABLE", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "ok"})
	})

	return r
}

func corsMiddleware(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && slices.Contains(allowed, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
