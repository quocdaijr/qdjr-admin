package adminapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/categories"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/profile"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/tags"
)

// Deps bundles cross-package dependencies the admin router needs. Adding a
// new admin resource means adding a field here (kept clean: handlers live in
// their own packages).
type Deps struct {
	Resolver        rbac.PermissionResolver
	PostsAdmin      posts.AdminWriter
	CategoriesAdmin categories.AdminWriter
	TagsAdmin       tags.AdminWriter
	ProfileAdmin    *profile.AdminRepository
}

// Register attaches admin endpoints to the given group. Plan 2 wires per-
// resource handlers (posts CRUD, etc.) alongside /me.
func Register(g *gin.RouterGroup, deps Deps) {
	g.GET("/me", meHandler(deps.Resolver))
	if deps.PostsAdmin != nil {
		posts.RegisterAdmin(g, deps.PostsAdmin, deps.Resolver)
	}
	if deps.CategoriesAdmin != nil {
		categories.RegisterAdmin(g, deps.CategoriesAdmin, deps.Resolver)
	}
	if deps.TagsAdmin != nil {
		tags.RegisterAdmin(g, deps.TagsAdmin, deps.Resolver)
	}
	if deps.ProfileAdmin != nil {
		profile.RegisterAdmin(g, deps.ProfileAdmin, deps.Resolver)
	}
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
