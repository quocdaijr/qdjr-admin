package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

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

// WrapMux returns a mux for connecthttp.Mount that registers every route on
// mux behind wrap, so middleware such as ConnectCORS sits in front of all the
// procedure paths Mount adds. Mount registers plain paths (no method), so
// CORS preflights reach the wrapper too.
func WrapMux(mux connecthttp.ServeMux, wrap func(http.Handler) http.Handler) connecthttp.ServeMux {
	return wrappedMux{mux: mux, wrap: wrap}
}

type wrappedMux struct {
	mux  connecthttp.ServeMux
	wrap func(http.Handler) http.Handler
}

func (m wrappedMux) Handle(pattern string, h http.Handler) { m.mux.Handle(pattern, m.wrap(h)) }

// RPCAuth verifies the bearer token and the permission for every call on the
// server it is installed on. A v2 interceptor wraps the whole call, unary and
// streaming alike, and runs before the request body is read, so a call with
// no valid token is rejected from its headers alone. Error messages are
// fixed text; causes stay server-side (RPCLog logs them).
func RPCAuth(v auth.Verifier, res rbac.PermissionResolver, perm string) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			info, ok := connect.CallInfoForServerContext(ctx)
			if !ok {
				return connect.NewError(connect.CodeInternal, "internal error")
			}
			ctx, err := authorize(ctx, v, res, perm, info.RequestHeader().Get("Authorization"))
			if err != nil {
				return err
			}
			return next(ctx, spec, stream)
		}
	}
}

// authorize returns ctx carrying the caller id, or the connect error to send.
func authorize(ctx context.Context, v auth.Verifier, res rbac.PermissionResolver, perm, header string) (context.Context, error) {
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, connect.NewError(connect.CodeUnauthenticated, "missing bearer token")
	}
	claims, err := v.Verify(strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "invalid or expired token").WithCause(err)
	}
	ok, err := res.Can(ctx, claims.UserID, perm)
	switch {
	case errors.Is(err, rbac.ErrNoRole):
		return nil, connect.NewError(connect.CodePermissionDenied, "no role assigned")
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, "permission check failed").WithCause(err)
	case !ok:
		return nil, connect.NewError(connect.CodePermissionDenied, "missing permission: "+perm)
	}
	return context.WithValue(ctx, ctxUserID, claims.UserID), nil
}

// RPCLog writes one line per call, unary or streaming: the procedure, the
// connect code ("ok" on success), the wall time (a stream's whole life), and
// on failure the error plus its server-side cause, which v2 never sends to
// the client. Mount it before RPCAuth so denied calls are logged too.
func RPCLog(logger *slog.Logger) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			start := time.Now()
			err := next(ctx, spec, stream)
			attrs := []any{
				"procedure", spec.Procedure,
				"code", "ok",
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if err != nil {
				attrs[3] = connect.CodeOf(err).String()
				attrs = append(attrs, "error", err.Error())
				if cause := errors.Unwrap(err); cause != nil {
					attrs = append(attrs, "cause", cause.Error())
				}
			}
			logger.InfoContext(ctx, "rpc", attrs...)
			return err
		}
	}
}

// RPCRecover turns a panic in anything it wraps into CodeInternal instead of
// a dropped connection (v1's WithRecover). http.ErrAbortHandler is re-panicked
// so net/http still aborts the response, as it must.
func RPCRecover() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) (err error) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}
				if e, ok := r.(error); ok && errors.Is(e, http.ErrAbortHandler) {
					panic(r)
				}
				err = connect.NewError(connect.CodeInternal, "internal error").
					WithCause(fmt.Errorf("panic in %s: %v", spec.Procedure, r))
			}()
			return next(ctx, spec, stream)
		}
	}
}
