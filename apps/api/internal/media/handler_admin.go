package media

import (
	"errors"
	"net/http"
	"regexp"
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

	maxFilenameLen = 255
	maxAltTextLen  = 500
	signedURLTTL   = 3600
)

// storagePathPattern enforces "media/<uuid><ext>" for /POST /media. The uuid
// is the v4 hex form (with dashes); ext is one of the canonical extensions.
var storagePathPattern = regexp.MustCompile(
	`^media/[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}\.(png|jpg|jpeg|webp|gif|svg)$`,
)

// RegisterAdmin mounts the admin media endpoints on group g (already wrapped
// with RequireAuth by the router).
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, storage Storage, res rbac.PermissionResolver) {
	g.GET("/media",
		apphttp.RequirePermission(res, "media:write"),
		adminListHandler(repo),
	)
	g.POST("/media/signed-url",
		apphttp.RequirePermission(res, "media:write"),
		adminSignedURLHandler(storage),
	)
	g.POST("/media",
		apphttp.RequirePermission(res, "media:write"),
		adminCreateHandler(repo),
	)
	g.DELETE("/media/:id",
		apphttp.RequirePermission(res, "media:write"),
		adminDeleteHandler(repo, storage, res),
	)
}

type signedURLPayload struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

type signedURLResponse struct {
	StoragePath string `json:"storage_path"`
	SignedURL   string `json:"signed_url"`
	ExpiresIn   int    `json:"expires_in"`
}

type adminCreatePayload struct {
	Filename    string  `json:"filename"`
	StoragePath string  `json:"storage_path"`
	MimeType    string  `json:"mime_type"`
	Size        int64   `json:"size"`
	Width       *int    `json:"width"`
	Height      *int    `json:"height"`
	AltText     *string `json:"alt_text"`
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

func adminSignedURLHandler(storage Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p signedURLPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		if l := len(p.Filename); l < 1 || l > maxFilenameLen {
			apphttp.Err(c, http.StatusBadRequest, "INVALID",
				"filename must be 1-255 chars")
			return
		}
		if !IsAllowedMime(p.MimeType) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID",
				"mime_type must be one of: image/png, image/jpeg, image/webp, image/gif, image/svg+xml")
			return
		}
		if p.Size <= 0 || p.Size > MaxUploadSize {
			apphttp.Err(c, http.StatusBadRequest, "INVALID",
				"size must be > 0 and <= 10 MB")
			return
		}

		storagePath := "media/" + uuid.New().String() + MimeExtensions[p.MimeType]
		signed, err := storage.SignedUploadURL(c.Request.Context(), storagePath)
		if err != nil {
			apphttp.Err(c, http.StatusBadGateway, "STORAGE", err.Error())
			return
		}
		apphttp.OK(c, signedURLResponse{
			StoragePath: storagePath,
			SignedURL:   signed,
			ExpiresIn:   signedURLTTL,
		})
	}
}

func adminCreateHandler(repo AdminWriter) gin.HandlerFunc {
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
		if l := len(p.Filename); l < 1 || l > maxFilenameLen {
			apphttp.Err(c, http.StatusBadRequest, "INVALID",
				"filename must be 1-255 chars")
			return
		}
		if !storagePathPattern.MatchString(p.StoragePath) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID",
				"storage_path must match media/<uuid>.<ext>")
			return
		}
		if !IsAllowedMime(p.MimeType) {
			apphttp.Err(c, http.StatusBadRequest, "INVALID",
				"mime_type must be one of: image/png, image/jpeg, image/webp, image/gif, image/svg+xml")
			return
		}
		if p.Size <= 0 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "size must be > 0")
			return
		}
		if p.Width != nil && *p.Width <= 0 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "width must be > 0")
			return
		}
		if p.Height != nil && *p.Height <= 0 {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "height must be > 0")
			return
		}
		if p.AltText != nil && len(*p.AltText) > maxAltTextLen {
			apphttp.Err(c, http.StatusBadRequest, "INVALID",
				"alt_text must be ≤500 chars")
			return
		}

		out, err := repo.Create(c.Request.Context(), CreateInput{
			Filename:    p.Filename,
			StoragePath: p.StoragePath,
			MimeType:    p.MimeType,
			Size:        p.Size,
			Width:       p.Width,
			Height:      p.Height,
			AltText:     p.AltText,
			UploadedBy:  uid,
		})
		if err != nil {
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
