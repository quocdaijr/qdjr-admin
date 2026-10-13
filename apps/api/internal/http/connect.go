package http

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"connectrpc.com/connect"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

// ConnectCORS is the net/http twin of corsMiddleware for the Connect mount,
// which sits in front of Gin on the outer mux. connect-web sends
// Connect-Protocol-Version on every call and Connect-Timeout-Ms when a
// timeout is set; nothing it reads back needs Access-Control-Expose-Headers.
// Preflights always get 204, listed origins also get the allow headers.
func ConnectCORS(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "POST,OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Connect-Protocol-Version,Connect-Timeout-Ms")
				h.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RPCAuth verifies the bearer token and the permission in one unary
// interceptor so every protocol the handler speaks (Connect, gRPC, gRPC-Web)
// gets a proper connect.Error instead of a JSON 401 body. connect-go v1 runs
// interceptors after unmarshal; fine for the small admin payloads here.
func RPCAuth(v auth.Verifier, res rbac.PermissionResolver, perm string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			h := req.Header().Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
			}
			claims, err := v.Verify(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			ok, err := res.Can(ctx, claims.UserID, perm)
			switch {
			case errors.Is(err, rbac.ErrNoRole):
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("no role assigned"))
			case err != nil:
				return nil, connect.NewError(connect.CodeInternal, err)
			case !ok:
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("missing permission: "+perm))
			}
			return next(context.WithValue(ctx, ctxUserID, claims.UserID), req)
		}
	}
}
