package contact

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

// RegisterPublic mounts POST /contact and starts the limiter's background
// evictor against context.Background() so the goroutine lives with the
// process. Rate-limiting check happens BEFORE JSON parsing so a flood of
// malformed bodies cannot bypass the budget.
func RegisterPublic(g *gin.RouterGroup, repo Writer, limiter *Limiter) {
	g.POST("/contact", postHandler(repo, limiter))
}

func postHandler(repo Writer, limiter *Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := clientIP(c)
		if !limiter.Allow(ip) {
			apphttp.Err(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}

		var in Input
		if err := c.ShouldBindJSON(&in); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "VALIDATION", "invalid JSON body")
			return
		}
		if err := in.Validate(); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "VALIDATION", err.Error())
			return
		}

		created, err := repo.Create(c.Request.Context(), CreateInput{
			Name:      in.Name,
			Email:     in.Email,
			Subject:   in.Subject,
			Body:      in.Body,
			IP:        ip,
			UserAgent: c.Request.UserAgent(),
		})
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.Created(c, gin.H{
			"id":         created.ID,
			"created_at": created.CreatedAt,
		})
	}
}

// clientIP extracts the requester IP, preferring proxy headers when present.
//
// Order: X-Forwarded-For (first hop), X-Real-IP, RemoteAddr. Returns "" if
// none can be parsed — callers should treat empty as "do not record".
func clientIP(c *gin.Context) string {
	if v := c.GetHeader("X-Forwarded-For"); v != "" {
		if i := strings.Index(v, ","); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	if v := c.GetHeader("X-Real-IP"); v != "" {
		return strings.TrimSpace(v)
	}
	host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	return host
}
