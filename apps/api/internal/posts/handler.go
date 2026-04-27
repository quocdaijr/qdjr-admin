package posts

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

// RegisterPublic mounts the public posts endpoints on the given group, which
// in production is the unauthenticated /v1 group.
func RegisterPublic(g *gin.RouterGroup, repo Reader) {
	g.GET("/posts", listHandler(repo))
	g.GET("/posts/:slug", getHandler(repo))
}

func listHandler(repo Reader) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := parsePositiveInt(c.Query("page"), defaultPage)
		perPage := parsePositiveInt(c.Query("perPage"), defaultPerPage)
		if perPage > maxPerPage {
			perPage = maxPerPage
		}

		f := ListFilter{
			Page:         page,
			PerPage:      perPage,
			CategorySlug: c.Query("category"),
			TagSlug:      c.Query("tag"),
			Q:            c.Query("q"),
		}

		posts, total, err := repo.ListPublished(c.Request.Context(), f)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.List(c, posts, apphttp.Meta{Page: page, PerPage: perPage, Total: total})
	}
}

func getHandler(repo Reader) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		post, err := repo.GetPublishedBySlug(c.Request.Context(), slug)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, post)
	}
}

// parsePositiveInt parses s as an int >= 1, falling back to fallback on any
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
