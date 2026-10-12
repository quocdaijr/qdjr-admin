package media

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"  // register decoders for DecodeConfig
	_ "image/jpeg" // (dimensions only)
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp" // webp DecodeConfig

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100

	maxFilenameLen = 255
	maxAltTextLen  = 500
	// multipartOverhead leaves room for boundaries and the alt_text field.
	multipartOverhead = 64 * 1024
	sniffLen          = 512
)

// RegisterAdmin mounts the admin media endpoints on group g (already wrapped
// with RequireAuth by the router).
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, storage Storage, res rbac.PermissionResolver) {
	g.GET("/media",
		apphttp.RequirePermission(res, "media:write"),
		adminListHandler(repo),
	)
	g.POST("/media",
		apphttp.RequirePermission(res, "media:write"),
		adminUploadHandler(repo, storage),
	)
	g.DELETE("/media/:id",
		apphttp.RequirePermission(res, "media:write"),
		adminDeleteHandler(repo, storage, res),
	)
}

func adminListHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := parsePositiveInt(c.Query("page"), defaultPage)
		perPage := parsePositiveInt(c.Query("perPage"), defaultPerPage)
		if perPage > maxPerPage {
			perPage = maxPerPage
		}
		out, total, err := repo.List(c.Request.Context(), AdminListFilter{
			Page: page, PerPage: perPage,
		})
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.List(c, out, apphttp.Meta{Page: page, PerPage: perPage, Total: total})
	}
}

// adminUploadHandler accepts multipart/form-data with a "file" part and an
// optional "alt_text" field. The type is detected from the bytes; only
// png/jpeg/webp/gif up to MaxUploadSize are stored.
func adminUploadHandler(repo AdminWriter, storage Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize+multipartOverhead)
		fh, err := c.FormFile("file")
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				apphttp.Err(c, http.StatusRequestEntityTooLarge, "TOO_LARGE", "file must be at most 10 MB")
				return
			}
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "multipart field \"file\" is required")
			return
		}
		if fh.Size > MaxUploadSize {
			apphttp.Err(c, http.StatusRequestEntityTooLarge, "TOO_LARGE", "file must be at most 10 MB")
			return
		}
		name := strings.TrimSpace(fh.Filename)
		if l := len(name); l < 1 || l > maxFilenameLen {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "filename must be 1-255 chars")
			return
		}
		var altText *string
		if v := strings.TrimSpace(c.PostForm("alt_text")); v != "" {
			if len(v) > maxAltTextLen {
				apphttp.Err(c, http.StatusBadRequest, "INVALID", "alt_text must be ≤500 chars")
				return
			}
			altText = &v
		}

		f, err := fh.Open()
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "cannot read upload")
			return
		}
		defer func() { _ = f.Close() }()
		data, err := io.ReadAll(io.LimitReader(f, MaxUploadSize+1))
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "cannot read upload")
			return
		}
		if int64(len(data)) > MaxUploadSize {
			apphttp.Err(c, http.StatusRequestEntityTooLarge, "TOO_LARGE", "file must be at most 10 MB")
			return
		}
		mime := http.DetectContentType(data[:min(len(data), sniffLen)])
		ext, allowed := MimeExtensions[mime]
		if len(data) == 0 || !allowed {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "file must be a PNG, JPEG, WebP or GIF image")
			return
		}
		var width, height *int
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			width, height = &cfg.Width, &cfg.Height
		}

		key := "media/" + uuid.New().String() + ext
		if err := storage.Put(c.Request.Context(), key, mime, bytes.NewReader(data), int64(len(data))); err != nil {
			apphttp.Err(c, http.StatusBadGateway, "STORAGE", err.Error())
			return
		}
		out, err := repo.Create(c.Request.Context(), CreateInput{
			Filename:    name,
			StoragePath: key,
			MimeType:    mime,
			Size:        int64(len(data)),
			Width:       width,
			Height:      height,
			AltText:     altText,
			UploadedBy:  uid,
		})
		if err != nil {
			_ = storage.Delete(c.Request.Context(), key) // don't orphan the object
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.Created(c, out)
	}
}

func adminDeleteHandler(repo AdminWriter, storage Storage, res rbac.PermissionResolver) gin.HandlerFunc {
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
		existing, err := repo.Get(c.Request.Context(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "media not found")
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}

		// Ownership: authors may only delete their own uploads. Users with
		// posts:read:all (editor / super_admin) bypass this check.
		elevated, err := res.Can(c.Request.Context(), uid, "posts:read:all")
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		if !elevated {
			if existing.UploadedBy == nil || *existing.UploadedBy != uid {
				apphttp.Err(c, http.StatusForbidden, "FORBIDDEN",
					"cannot delete another user's media")
				return
			}
		}

		// Best-effort storage delete BEFORE removing the DB row so that a
		// transient storage error doesn't leave orphan files. Storage 404 is
		// treated as success by the client.
		if err := storage.Delete(c.Request.Context(), existing.StoragePath); err != nil {
			apphttp.Err(c, http.StatusBadGateway, "STORAGE", err.Error())
			return
		}
		if err := repo.Delete(c.Request.Context(), id); err != nil {
			if errors.Is(err, ErrNotFound) {
				apphttp.Err(c, http.StatusNotFound, "NOT_FOUND", "media not found")
				return
			}
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.NoContent(c)
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
