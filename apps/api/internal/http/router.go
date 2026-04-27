// apps/api/internal/http/router.go
package http

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
)

// RouterDeps wires runtime dependencies into the HTTP layer.
type RouterDeps struct {
	Pool        *pgxpool.Pool
	CORSOrigins []string
	Verifier    auth.Verifier
	// RegisterAdmin attaches admin handlers to the protected /v1/admin group.
	// Kept as a callback to avoid an import cycle between http and adminapi.
	RegisterAdmin func(*gin.RouterGroup)
	// RegisterPublic attaches unauthenticated /v1 handlers (used by Plan 2).
	RegisterPublic func(*gin.RouterGroup)
}

// NewRouter builds the Gin engine.
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

	v1 := r.Group("/v1")
	if deps.RegisterPublic != nil {
		deps.RegisterPublic(v1)
	}
	if deps.RegisterAdmin != nil && deps.Verifier != nil {
		admin := v1.Group("/admin", RequireAuth(deps.Verifier))
		deps.RegisterAdmin(admin)
	}

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
