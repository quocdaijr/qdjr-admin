package categories

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// RegisterAdmin mounts the admin categories CRUD on group g (already wrapped
// in RequireAuth by the router). Per-route taxonomy:write checks added inline.
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, res rbac.PermissionResolver) {
	g.GET("/categories",
		apphttp.RequirePermission(res, "taxonomy:write"),
		adminListHandler(repo),
	)
	g.POST("/categories",
		apphttp.RequirePermission(res, "taxonomy:write"),
		adminCreateHandler(repo),
	)
	g.GET("/categories/:id",
		apphttp.RequirePermission(res, "taxonomy:write"),
		adminGetHandler(repo),
	)
	g.PATCH("/categories/:id",
		apphttp.RequirePermission(res, "taxonomy:write"),
		adminUpdateHandler(repo),
	)
	g.DELETE("/categories/:id",
		apphttp.RequirePermission(res, "taxonomy:write"),
		adminDeleteHandler(repo),
	)
}

type adminCreatePayload struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type adminUpdatePayload struct {
	Slug        *string `json:"slug,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func adminListHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := parsePositiveInt(c.Query("page"), defaultPage)
		perPage := parsePositiveInt(c.Query("perPage"), defaultPerPage)
		if perPage > maxPerPage {
			perPage = maxPerPage
		}
		out, total, err := repo.List(c.Request.Context(), AdminListFilter{
			Page: page, PerPage: perPage, Q: c.Query("q"),
		})
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.List(c, out, apphttp.Meta{Page: page, PerPage: perPage, Total: total})
	}
}

func adminCreateHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p adminCreatePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		if l := len(p.Name); l < 1 || l > 200 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "name must be 1-200 chars")
			return
		}
		if p.Description != nil && len(*p.Description) > 1000 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "description must be ≤1000 chars")
			return
		}
		slug := p.Slug
		if slug == "" {
			slug = posts.Slugify(p.Name)
		}
		if !posts.ValidSlug(slug) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "slug must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be ≤200 chars")
			return
		}
		out, err := repo.Create(c.Request.Context(), CreateInput{
			Slug: slug, Name: p.Name, Description: p.Description,
		})
		switch {
		case errors.Is(err, ErrSlugConflict):
			apphttp.Err(c, http.StatusConflict, "CONFLICT", "slug already exists")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.Created(c, out)
	}
}

func adminGetHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}
		cat, err := repo.GetByID(c.Request.Context(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "category not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, cat)
	}
}

func adminUpdateHandler(repo AdminWriter) gin.HandlerFunc {
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
		if p.Name != nil {
			if l := len(*p.Name); l < 1 || l > 200 {
				apphttp.Err(c, http.StatusBadRequest, "INVALID", "name must be 1-200 chars")
				return
			}
		}
		if p.Description != nil && len(*p.Description) > 1000 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "description must be ≤1000 chars")
			return
		}
		if p.Slug != nil && !posts.ValidSlug(*p.Slug) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "slug must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be ≤200 chars")
			return
		}
		out, err := repo.Update(c.Request.Context(), id, UpdateInput{
			Slug: p.Slug, Name: p.Name, Description: p.Description,
		})
		switch {
		case errors.Is(err, ErrSlugConflict):
			apphttp.Err(c, http.StatusConflict, "CONFLICT", "slug already exists")
			return
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "category not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}

func adminDeleteHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}
		err = repo.Delete(c.Request.Context(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "category not found")
			return
		case errors.Is(err, ErrInUse):
			apphttp.Err(c, http.StatusConflict, "IN_USE", err.Error())
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.NoContent(c)
	}
}
