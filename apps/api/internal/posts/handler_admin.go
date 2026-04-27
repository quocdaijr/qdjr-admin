package posts

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// RegisterAdmin mounts the admin posts CRUD on group g (already wrapped in
// RequireAuth by the router). Per-route permission checks are added inline.
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, res rbac.PermissionResolver) {
	g.GET("/posts", adminListHandler(repo, res))
	g.POST("/posts", apphttp.RequirePermission(res, "posts:write"), adminCreateHandler(repo, res))
	g.GET("/posts/:id", adminGetHandler(repo, res))
	g.PATCH("/posts/:id",
		apphttp.RequirePermission(res, "posts:write"),
		adminUpdateHandler(repo, res),
	)
	g.DELETE("/posts/:id",
		apphttp.RequirePermission(res, "posts:write"),
		adminDeleteHandler(repo, res),
	)
	g.POST("/posts/:id/publish",
		apphttp.RequirePermission(res, "posts:publish"),
		adminPublishHandler(repo, true),
	)
	g.POST("/posts/:id/unpublish",
		apphttp.RequirePermission(res, "posts:publish"),
		adminPublishHandler(repo, false),
	)
}

// adminCreatePayload mirrors the Create body. Validation is done in the
// handler so we can apply per-field bounds + the slug regex consistently.
type adminCreatePayload struct {
	Title           string      `json:"title"`
	Slug            string      `json:"slug"`
	Excerpt         *string     `json:"excerpt"`
	Content         string      `json:"content"`
	Status          string      `json:"status"`
	ThumbnailID     *uuid.UUID  `json:"thumbnail_id"`
	OGImageID       *uuid.UUID  `json:"og_image_id"`
	MetaTitle       *string     `json:"meta_title"`
	MetaDescription *string     `json:"meta_description"`
	CategoryIDs     []uuid.UUID `json:"category_ids"`
	TagIDs          []uuid.UUID `json:"tag_ids"`
}

type adminUpdatePayload struct {
	Title           *string      `json:"title,omitempty"`
	Slug            *string      `json:"slug,omitempty"`
	Excerpt         *string      `json:"excerpt,omitempty"`
	Content         *string      `json:"content,omitempty"`
	Status          *string      `json:"status,omitempty"`
	ThumbnailID     *uuid.UUID   `json:"thumbnail_id,omitempty"`
	OGImageID       *uuid.UUID   `json:"og_image_id,omitempty"`
	MetaTitle       *string      `json:"meta_title,omitempty"`
	MetaDescription *string      `json:"meta_description,omitempty"`
	CategoryIDs     *[]uuid.UUID `json:"category_ids,omitempty"`
	TagIDs          *[]uuid.UUID `json:"tag_ids,omitempty"`
}

func adminListHandler(repo AdminWriter, res rbac.PermissionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		page := parsePositiveInt(c.Query("page"), defaultPage)
		perPage := parsePositiveInt(c.Query("perPage"), defaultPerPage)
		if perPage > maxPerPage {
			perPage = maxPerPage
		}

		f := AdminListFilter{
			Page:    page,
			PerPage: perPage,
			Status:  c.Query("status"),
			Q:       c.Query("q"),
		}

		// Author role: scope listing to own posts.
		elevated, err := canAccessAnyPost(c.Request.Context(), res, uid)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		if !elevated {
			id := uid
			f.CreatedBy = &id
		}

		posts, total, err := repo.List(c.Request.Context(), f)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.List(c, posts, apphttp.Meta{Page: page, PerPage: perPage, Total: total})
	}
}

func adminCreateHandler(repo AdminWriter, res rbac.PermissionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		var p adminCreatePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		// Title required, content required.
		if l := len(p.Title); l < 1 || l > 300 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "title must be 1-300 chars")
			return
		}
		if p.Content == "" {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "content is required")
			return
		}
		if p.Excerpt != nil && len(*p.Excerpt) > 1000 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "excerpt must be ≤1000 chars")
			return
		}
		if p.MetaTitle != nil && len(*p.MetaTitle) > 200 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "meta_title must be ≤200 chars")
			return
		}
		if p.MetaDescription != nil && len(*p.MetaDescription) > 500 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "meta_description must be ≤500 chars")
			return
		}

		// Slug auto/validate.
		slug := p.Slug
		if slug == "" {
			slug = slugify(p.Title)
		}
		if !validSlug(slug) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "slug must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be ≤200 chars")
			return
		}

		// Status default + publish-permission downgrade.
		status := p.Status
		if status == "" {
			status = "draft"
		}
		if status != "draft" && status != "published" {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "status must be 'draft' or 'published'")
			return
		}
		if status == "published" {
			canPub, err := res.Can(c.Request.Context(), uid, "posts:publish")
			if err != nil {
				apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			if !canPub {
				status = "draft"
			}
		}

		out, err := repo.Create(c.Request.Context(), CreateInput{
			Title:           p.Title,
			Slug:            slug,
			Excerpt:         p.Excerpt,
			Content:         p.Content,
			Status:          status,
			ThumbnailID:     p.ThumbnailID,
			OGImageID:       p.OGImageID,
			MetaTitle:       p.MetaTitle,
			MetaDescription: p.MetaDescription,
			CategoryIDs:     p.CategoryIDs,
			TagIDs:          p.TagIDs,
			CreatedBy:       uid,
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

func adminGetHandler(repo AdminWriter, res rbac.PermissionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}
		post, err := repo.GetByID(c.Request.Context(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		if err := requireOwnedOrElevated(c.Request.Context(), res, uid, post); err != nil {
			if errors.Is(err, ErrForbidden) {
				apphttp.Err(c, http.StatusForbidden, "FORBIDDEN", "cannot access another author's post")
				return
			}
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, post)
	}
}

func adminUpdateHandler(repo AdminWriter, res rbac.PermissionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
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

		// Field-level validation.
		if p.Title != nil {
			if l := len(*p.Title); l < 1 || l > 300 {
				apphttp.Err(c, http.StatusBadRequest, "INVALID", "title must be 1-300 chars")
				return
			}
		}
		if p.Excerpt != nil && len(*p.Excerpt) > 1000 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "excerpt must be ≤1000 chars")
			return
		}
		if p.MetaTitle != nil && len(*p.MetaTitle) > 200 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "meta_title must be ≤200 chars")
			return
		}
		if p.MetaDescription != nil && len(*p.MetaDescription) > 500 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "meta_description must be ≤500 chars")
			return
		}
		if p.Slug != nil && !validSlug(*p.Slug) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "slug must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be ≤200 chars")
			return
		}
		if p.Status != nil {
			if *p.Status != "draft" && *p.Status != "published" && *p.Status != "archived" {
				apphttp.Err(c, http.StatusBadRequest, "INVALID", "status must be 'draft', 'published', or 'archived'")
				return
			}
		}

		// Load existing for ownership + transition checks.
		existing, err := repo.GetByID(c.Request.Context(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		if err := requireOwnedOrElevated(c.Request.Context(), res, uid, existing); err != nil {
			if errors.Is(err, ErrForbidden) {
				apphttp.Err(c, http.StatusForbidden, "FORBIDDEN", "cannot modify another author's post")
				return
			}
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}

		// Status → published transition requires posts:publish.
		if p.Status != nil && *p.Status == "published" && existing.Status != "published" {
			canPub, err := res.Can(c.Request.Context(), uid, "posts:publish")
			if err != nil {
				apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			if !canPub {
				apphttp.Err(c, http.StatusForbidden, "FORBIDDEN", "missing permission: posts:publish")
				return
			}
		}

		out, err := repo.Update(c.Request.Context(), id, UpdateInput{
			Title:           p.Title,
			Slug:            p.Slug,
			Excerpt:         p.Excerpt,
			Content:         p.Content,
			Status:          p.Status,
			ThumbnailID:     p.ThumbnailID,
			OGImageID:       p.OGImageID,
			MetaTitle:       p.MetaTitle,
			MetaDescription: p.MetaDescription,
			CategoryIDs:     p.CategoryIDs,
			TagIDs:          p.TagIDs,
			UpdatedBy:       uid,
		})
		switch {
		case errors.Is(err, ErrSlugConflict):
			apphttp.Err(c, http.StatusConflict, "CONFLICT", "slug already exists")
			return
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}

func adminDeleteHandler(repo AdminWriter, res rbac.PermissionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}
		existing, err := repo.GetByID(c.Request.Context(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		if err := requireOwnedOrElevated(c.Request.Context(), res, uid, existing); err != nil {
			if errors.Is(err, ErrForbidden) {
				apphttp.Err(c, http.StatusForbidden, "FORBIDDEN", "cannot delete another author's post")
				return
			}
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		if err := repo.Delete(c.Request.Context(), id); err != nil {
			if errors.Is(err, ErrNotFound) {
				apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "post not found")
				return
			}
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.NoContent(c)
	}
}

func adminPublishHandler(repo AdminWriter, publish bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid id")
			return
		}
		out, err := repo.SetPublished(c.Request.Context(), id, publish, uid)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}
