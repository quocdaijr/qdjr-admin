package contact

import (
	"errors"
	"net/http"
	"strconv"

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

// RegisterAdmin mounts the admin contact-messages endpoints on group g.
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, res rbac.PermissionResolver) {
	g.GET("/contact-messages",
		apphttp.RequirePermission(res, "contact:read"),
		adminListHandler(repo),
	)
	g.PATCH("/contact-messages/:id",
		apphttp.RequirePermission(res, "contact:write"),
		adminUpdateStatusHandler(repo),
	)
}

type adminUpdatePayload struct {
	Status string `json:"status"`
}

func adminListHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := parsePositiveInt(c.Query("page"), defaultPage)
		perPage := parsePositiveInt(c.Query("perPage"), defaultPerPage)
		if perPage > maxPerPage {
			perPage = maxPerPage
		}
		status := c.Query("status")
		if status != "" && !IsValidStatus(status) {
			apphttp.Err(c, http.StatusBadRequest, "VALIDATION", "invalid status filter")
			return
		}
		out, total, err := repo.List(c.Request.Context(), AdminListFilter{
			Page: page, PerPage: perPage, Status: status,
		})
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.List(c, out, apphttp.Meta{Page: page, PerPage: perPage, Total: total})
	}
}

func adminUpdateStatusHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}
		var p adminUpdatePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		if !IsValidStatus(p.Status) {
			apphttp.Err(c, http.StatusBadRequest, "VALIDATION", "status must be one of: new, read, replied, spam")
			return
		}
		out, err := repo.UpdateStatus(c.Request.Context(), id, p.Status)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "contact message not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}

// parsePositiveInt parses s as int >= 1, falling back to fallback on any
// parse error or non-positive value.
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
