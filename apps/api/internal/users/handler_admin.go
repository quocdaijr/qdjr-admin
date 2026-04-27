package users

import (
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

// RegisterAdmin mounts the admin user-management endpoints on group g (which
// already has RequireAuth applied). All routes require the users:manage
// permission.
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, supa SupabaseAdmin, res rbac.PermissionResolver) {
	g.GET("/users",
		apphttp.RequirePermission(res, "users:manage"),
		adminListHandler(repo),
	)
	g.POST("/users",
		apphttp.RequirePermission(res, "users:manage"),
		adminInviteHandler(repo, supa),
	)
	g.PATCH("/users/:id/role",
		apphttp.RequirePermission(res, "users:manage"),
		adminSetRoleHandler(repo),
	)
	g.DELETE("/users/:id",
		apphttp.RequirePermission(res, "users:manage"),
		adminDeleteHandler(repo, supa),
	)
}

type adminInvitePayload struct {
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password,omitempty"`
}

type adminRolePayload struct {
	Role string `json:"role"`
}

func adminListHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := parsePositiveInt(c.Query("page"), defaultPage)
		perPage := parsePositiveInt(c.Query("perPage"), defaultPerPage)
		if perPage > maxPerPage {
			perPage = maxPerPage
		}
		out, total, err := repo.List(c.Request.Context(), ListFilter{
			Page: page, PerPage: perPage,
		})
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.List(c, out, apphttp.Meta{Page: page, PerPage: perPage, Total: total})
	}
}

func adminInviteHandler(repo AdminWriter, supa SupabaseAdmin) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p adminInvitePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		email := strings.TrimSpace(p.Email)
		if email == "" {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "email is required")
			return
		}
		if _, err := mail.ParseAddress(email); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "email is not a valid address")
			return
		}
		if !IsAllowedRole(p.Role) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "role must be one of super_admin, editor, author")
			return
		}

		uid, err := supa.EnsureUser(c.Request.Context(), email, p.Password)
		if err != nil {
			apphttp.Err(c, http.StatusBadGateway, "BAD_GATEWAY", err.Error())
			return
		}

		if err := repo.SetRole(c.Request.Context(), uid, p.Role); err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}

		out, err := repo.Get(c.Request.Context(), uid)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.Created(c, out)
	}
}

func adminSetRoleHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}
		var p adminRolePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		if !IsAllowedRole(p.Role) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "role must be one of super_admin, editor, author")
			return
		}

		caller, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}

		if id == caller && p.Role != "super_admin" {
			last, err := repo.IsLastSuperAdmin(c.Request.Context(), caller)
			if err != nil {
				apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			if last {
				apphttp.Err(c, http.StatusConflict, "LAST_SUPER_ADMIN",
					"cannot demote the last super_admin")
				return
			}
		}

		if err := repo.SetRole(c.Request.Context(), id, p.Role); err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}

		out, err := repo.Get(c.Request.Context(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}

func adminDeleteHandler(repo AdminWriter, supa SupabaseAdmin) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}

		caller, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}

		if id == caller {
			last, err := repo.IsLastSuperAdmin(c.Request.Context(), caller)
			if err != nil {
				apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			if last {
				apphttp.Err(c, http.StatusConflict, "LAST_SUPER_ADMIN",
					"cannot delete the last super_admin")
				return
			}
		}

		if err := supa.DeleteUser(c.Request.Context(), id); err != nil {
			apphttp.Err(c, http.StatusBadGateway, "BAD_GATEWAY", err.Error())
			return
		}
		// FK cascades, but call DeleteRole defensively (no-op if absent).
		if err := repo.DeleteRole(c.Request.Context(), id); err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.NoContent(c)
	}
}

// parsePositiveInt parses s as int >= 1, falling back on parse error or
// non-positive values.
func parsePositiveInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
