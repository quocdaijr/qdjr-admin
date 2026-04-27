package sitesettings

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
)

// RegisterPublic mounts the public site-settings endpoint on the given group.
func RegisterPublic(g *gin.RouterGroup, repo Reader) {
	g.GET("/site-settings", getHandler(repo))
}

func getHandler(repo Reader) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, err := repo.GetPublic(c.Request.Context())
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		apphttp.OK(c, s)
	}
}
