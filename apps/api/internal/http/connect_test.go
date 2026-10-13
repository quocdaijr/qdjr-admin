package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

const connectTestSecret = "connect-secret-connect-secret-1234"

type permStub struct {
	perms map[string]bool
	err   error
}

func (p *permStub) Role(context.Context, uuid.UUID) (string, error) { return "editor", p.err }
func (p *permStub) Permissions(context.Context, uuid.UUID) ([]string, error) {
	return nil, p.err
}
func (p *permStub) Can(_ context.Context, _ uuid.UUID, perm string) (bool, error) {
	return p.perms[perm], p.err
}

func signedToken(t *testing.T, uid uuid.UUID, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uid.String(),
		"exp": exp.Unix(),
	})
	s, err := tok.SignedString([]byte(connectTestSecret))
	require.NoError(t, err)
	return s
}

// callThrough runs the interceptor with a next that records the ctx user id.
func callThrough(t *testing.T, i connect.Interceptor, authHeader string) (uuid.UUID, bool, error) {
	t.Helper()
	var seen uuid.UUID
	var ok bool
	next := func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		seen, ok = UserIDFrom(ctx)
		return connect.NewResponse(&struct{}{}), nil
	}
	req := connect.NewRequest(&struct{}{})
	if authHeader != "" {
		req.Header().Set("Authorization", authHeader)
	}
	_, err := i.WrapUnary(next)(context.Background(), req)
	return seen, ok, err
}

func TestRPCAuth_MissingToken_Unauthenticated(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{"lessons:write": true}}, "lessons:write")
	_, _, err := callThrough(t, i, "")
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPCAuth_ExpiredToken_Unauthenticated(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{"lessons:write": true}}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(-time.Minute)))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPCAuth_MissingPermission_PermissionDenied(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{}}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(time.Hour)))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPCAuth_NoRole_PermissionDenied(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{err: rbac.ErrNoRole}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(time.Hour)))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPCAuth_ResolverError_Internal(t *testing.T) {
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{err: errors.New("db down")}, "lessons:write")
	_, _, err := callThrough(t, i, "Bearer "+signedToken(t, uuid.New(), time.Now().Add(time.Hour)))
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestRPCAuth_OK_PutsUserInContext(t *testing.T) {
	uid := uuid.New()
	i := RPCAuth(auth.NewHS256Verifier([]byte(connectTestSecret)), &permStub{perms: map[string]bool{"lessons:write": true}}, "lessons:write")
	seen, ok, err := callThrough(t, i, "Bearer "+signedToken(t, uid, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, uid, seen)
}

func TestConnectCORS_PreflightAllowed(t *testing.T) {
	h := ConnectCORS([]string{"https://admin.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/lessons.v1.LessonsService/ListCourses", nil)
	req.Header.Set("Origin", "https://admin.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "https://admin.example", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Connect-Protocol-Version")
	assert.Equal(t, "Origin", rr.Header().Get("Vary"))
}

func TestConnectCORS_PreflightUnlistedOrigin(t *testing.T) {
	h := ConnectCORS([]string{"https://admin.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/lessons.v1.LessonsService/ListCourses", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestConnectCORS_PassesPostThrough(t *testing.T) {
	h := ConnectCORS([]string{"https://admin.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodPost, "/lessons.v1.LessonsService/ListCourses", nil)
	req.Header.Set("Origin", "https://admin.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTeapot, rr.Code)
	assert.Equal(t, "https://admin.example", rr.Header().Get("Access-Control-Allow-Origin"))
}
