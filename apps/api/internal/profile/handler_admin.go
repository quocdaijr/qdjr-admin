package profile

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// RegisterAdmin mounts the admin profile endpoints on group g (already
// wrapped in RequireAuth by the router).
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, res rbac.PermissionResolver) {
	g.GET("/profile",
		apphttp.RequirePermission(res, "profile:write"),
		adminGetHandler(repo),
	)
	g.PATCH("/profile",
		apphttp.RequirePermission(res, "profile:write"),
		adminUpdateHandler(repo),
	)
}

// adminUpdatePayload is the PATCH body. AvatarID uses json.RawMessage so we
// can distinguish "missing key" from "explicit null".
type adminUpdatePayload struct {
	FullName    *string            `json:"full_name,omitempty"`
	Bio         *string            `json:"bio,omitempty"`
	AvatarID    json.RawMessage    `json:"avatar_id,omitempty"`
	Tagline     *string            `json:"tagline,omitempty"`
	SocialLinks *map[string]string `json:"social_links,omitempty"`
	Location    *string            `json:"location,omitempty"`
	Email       *string            `json:"email,omitempty"`
}

// parseAvatarID inspects the raw JSON for avatar_id. Returns:
//
//	(nil, false, nil) when the key was absent.
//	(set, true, nil)  when present (set itself is *uuid.UUID, possibly nil for null).
//	(nil, false, err) on parse failure.
func parseAvatarID(raw json.RawMessage) (*uuid.UUID, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false, err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, false, err
	}
	return &id, true, nil
}

func adminGetHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := repo.Get(c.Request.Context())
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, p)
	}
}

func adminUpdateHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		var p adminUpdatePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		avatarValue, avatarSet, err := parseAvatarID(p.AvatarID)
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "invalid avatar_id: "+err.Error())
			return
		}
		in := UpdateInput{
			FullName:    p.FullName,
			Bio:         p.Bio,
			Tagline:     p.Tagline,
			SocialLinks: p.SocialLinks,
			Location:    p.Location,
			Email:       p.Email,
			UpdatedBy:   uid,
		}
		if avatarSet {
			in.AvatarID = &avatarValue
		}
		out, err := repo.Update(c.Request.Context(), in)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}
