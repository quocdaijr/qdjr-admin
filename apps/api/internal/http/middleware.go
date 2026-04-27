package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

type ctxKey string

const (
	ctxUserID ctxKey = "uid"
	ctxRole   ctxKey = "role"
)

// RequireAuth parses the Bearer JWT, verifies it, and stores user id + email
// in the request context.
func RequireAuth(v auth.Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "missing bearer token")
			return
		}
		claims, err := v.Verify(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", err.Error())
			return
		}
		ctx := context.WithValue(c.Request.Context(), ctxUserID, claims.UserID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequirePermission denies the request unless the resolver says the user has perm.
// On a missing role (ErrNoRole) returns 403 (treat as no permissions).
func RequirePermission(res rbac.PermissionResolver, perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := UserIDFromContext(c)
		if !ok {
			Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		ok, err := res.Can(c.Request.Context(), uid, perm)
		switch {
		case errors.Is(err, rbac.ErrNoRole):
			Err(c, http.StatusForbidden, "FORBIDDEN", "no role assigned")
			return
		case err != nil:
			Err(c, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		case !ok:
			Err(c, http.StatusForbidden, "FORBIDDEN", "missing permission: "+perm)
			return
		}
		c.Next()
	}
}

// UserIDFromContext returns the authenticated caller's id (set by RequireAuth).
func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v := c.Request.Context().Value(ctxUserID)
	id, ok := v.(uuid.UUID)
	return id, ok
}
