package sitesettings

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// RegisterAdmin mounts the admin site-settings endpoints on group g.
func RegisterAdmin(g *gin.RouterGroup, repo AdminWriter, res rbac.PermissionResolver) {
	g.GET("/site-settings",
		apphttp.RequirePermission(res, "settings:write"),
		adminGetHandler(repo),
	)
	g.PATCH("/site-settings",
		apphttp.RequirePermission(res, "settings:write"),
		adminUpdateHandler(repo),
	)
}

type adminUpdatePayload struct {
	SiteTitle       *string            `json:"site_title,omitempty"`
	SiteDescription *string            `json:"site_description,omitempty"`
	FooterText      *string            `json:"footer_text,omitempty"`
	ContactEmail    *string            `json:"contact_email,omitempty"`
	SocialLinks     *map[string]string `json:"social_links,omitempty"`
}

func adminGetHandler(repo AdminWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, err := repo.Get(c.Request.Context())
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, s)
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
		if p.SiteTitle != nil {
			if l := len(*p.SiteTitle); l < 1 || l > 200 {
				apphttp.Err(c, http.StatusBadRequest, "INVALID", "site_title must be 1-200 chars")
				return
			}
		}
		out, err := repo.Update(c.Request.Context(), UpdateInput{
			SiteTitle:       p.SiteTitle,
			SiteDescription: p.SiteDescription,
			FooterText:      p.FooterText,
			ContactEmail:    p.ContactEmail,
			SocialLinks:     p.SocialLinks,
			UpdatedBy:       uid,
		})
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, out)
	}
}
