package adminapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// Register attaches admin endpoints to the given group. Plan 1 only registers
// /me; Plan 2 expands this with per-resource handlers.
func Register(g *gin.RouterGroup, res rbac.PermissionResolver) {
	g.GET("/me", meHandler(res))
}

func meHandler(res rbac.PermissionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		role, err := res.Role(c.Request.Context(), uid)
		switch {
		case errors.Is(err, rbac.ErrNoRole):
			apphttp.Err(c, http.StatusForbidden, "FORBIDDEN", "no role assigned")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		perms, err := res.Permissions(c.Request.Context(), uid)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, gin.H{
			"user_id":     uid.String(),
			"role":        role,
			"permissions": perms,
		})
	}
}
