// Package session serves the admin login, token refresh and password change
// endpoints on top of auth (tokens, bcrypt) and users (the account store).
package session

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/contact"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/users"
)

// Store is the account lookup the handlers need (users.PGAccounts).
type Store interface {
	FindByEmail(ctx context.Context, email string) (users.Credential, error)
	FindByID(ctx context.Context, id uuid.UUID) (users.Credential, error)
	TouchSignIn(ctx context.Context, id uuid.UUID) error
	SetPassword(ctx context.Context, id uuid.UUID, hash string) error
}

type loginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshPayload struct {
	RefreshToken string `json:"refresh_token"`
}

type passwordPayload struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type sessionUser struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

type loginResponse struct {
	auth.TokenPair
	User sessionUser `json:"user"`
}

// RegisterPublic mounts POST /auth/login and POST /auth/refresh on g (the
// unauthenticated /v1 group). Both share the per-IP limiter.
func RegisterPublic(g *gin.RouterGroup, store Store, iss *auth.Issuer, limiter *contact.Limiter) {
	g.POST("/auth/login", limited(limiter), loginHandler(store, iss))
	g.POST("/auth/refresh", limited(limiter), refreshHandler(store, iss))
}

// RegisterAdmin mounts PATCH /me/password on the authenticated admin group.
func RegisterAdmin(g *gin.RouterGroup, store Store) {
	g.PATCH("/me/password", passwordHandler(store))
}

func limited(l *contact.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(contact.ClientIP(c)) {
			apphttp.Err(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many attempts, try again later")
			return
		}
		c.Next()
	}
}

func invalidCredentials(c *gin.Context) {
	apphttp.Err(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
}

func loginHandler(store Store, iss *auth.Issuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p loginPayload
		if err := c.ShouldBindJSON(&p); err != nil || strings.TrimSpace(p.Email) == "" || p.Password == "" {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "email and password are required")
			return
		}
		email := strings.ToLower(strings.TrimSpace(p.Email))
		cred, err := store.FindByEmail(c.Request.Context(), email)
		switch {
		case errors.Is(err, users.ErrNotFound):
			auth.CheckPassword(auth.DummyHash, p.Password) // same cost as a real miss
			invalidCredentials(c)
			return
		case err != nil:
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", "login failed")
			return
		}
		if !auth.CheckPassword(cred.PasswordHash, p.Password) {
			invalidCredentials(c)
			return
		}
		pair, err := iss.Issue(cred.ID, cred.Email)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", "login failed")
			return
		}
		_ = store.TouchSignIn(c.Request.Context(), cred.ID) // informational only
		apphttp.OK(c, loginResponse{TokenPair: pair, User: sessionUser{ID: cred.ID, Email: cred.Email}})
	}
}

func refreshHandler(store Store, iss *auth.Issuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p refreshPayload
		if err := c.ShouldBindJSON(&p); err != nil || p.RefreshToken == "" {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", "refresh_token is required")
			return
		}
		claims, err := iss.VerifyRefresh(p.RefreshToken)
		if err != nil {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "invalid refresh token")
			return
		}
		// Re-read the account so a deleted user cannot keep a session alive.
		cred, err := store.FindByID(c.Request.Context(), claims.UserID)
		if err != nil {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "account no longer exists")
			return
		}
		pair, err := iss.Issue(cred.ID, cred.Email)
		if err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", "refresh failed")
			return
		}
		apphttp.OK(c, loginResponse{TokenPair: pair, User: sessionUser{ID: cred.ID, Email: cred.Email}})
	}
}

func passwordHandler(store Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := apphttp.UserIDFromContext(c)
		if !ok {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "auth required")
			return
		}
		var p passwordPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		cred, err := store.FindByID(c.Request.Context(), uid)
		if err != nil {
			apphttp.Err(c, http.StatusUnauthorized, "UNAUTHENTICATED", "account no longer exists")
			return
		}
		if !auth.CheckPassword(cred.PasswordHash, p.CurrentPassword) {
			apphttp.Err(c, http.StatusBadRequest, "WRONG_PASSWORD", "current password is incorrect")
			return
		}
		hash, err := auth.HashPassword(p.NewPassword)
		if err != nil {
			apphttp.Err(c, http.StatusBadRequest, "INVALID", err.Error())
			return
		}
		if err := store.SetPassword(c.Request.Context(), uid, hash); err != nil {
			apphttp.Err(c, http.StatusInternalServerError, "INTERNAL", "could not change password")
			return
		}
		apphttp.NoContent(c)
	}
}
