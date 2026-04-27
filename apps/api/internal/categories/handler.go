package categories

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
)

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

// RegisterPublic mounts the public categories endpoints on the given group.
//
// postsReader is used to resolve the posts-by-category endpoint without
// duplicating the post listing logic.
func RegisterPublic(g *gin.RouterGroup, repo Reader, postsReader posts.Reader) {
	g.GET("/categories", listHandler(repo))
	g.GET("/categories/:slug/posts", postsBySlugHandler(repo, postsReader))
}

func listHandler(repo Reader) gin.HandlerFunc {
	return func(c *gin.Context) {
		out, err := repo.List(c.Request.Context())
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}

func postsBySlugHandler(repo Reader, postsReader posts.Reader) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if _, err := repo.GetBySlug(c.Request.Context(), slug); err != nil {
			if errors.Is(err, ErrNotFound) {
				apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "category not found")
				return
			}
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}

		page := parsePositiveInt(c.Query("page"), defaultPage)
		perPage := parsePositiveInt(c.Query("perPage"), defaultPerPage)
		if perPage > maxPerPage {
			perPage = maxPerPage
		}

		f := posts.ListFilter{
			Page:         page,
			PerPage:      perPage,
			CategorySlug: slug,
			Q:            c.Query("q"),
		}
		out, total, err := postsReader.ListPublished(c.Request.Context(), f)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.List(c, out, apphttp.Meta{Page: page, PerPage: perPage, Total: total})
	}
}

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
