package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

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

// RPCAuth verifies the bearer token and the permission for every call the
// handler serves, unary and streaming, so each protocol (Connect, gRPC,
// gRPC-Web) gets a proper connect.Error instead of a JSON 401 body. It must be
// a full Interceptor: a UnaryInterceptorFunc passes streaming calls through
// untouched, which would leave a stream like WatchCourse unauthenticated.
func RPCAuth(v auth.Verifier, res rbac.PermissionResolver, perm string) connect.Interceptor {
	return &rpcAuth{verifier: v, resolver: res, perm: perm}
}

type rpcAuth struct {
	verifier auth.Verifier
	resolver rbac.PermissionResolver
	perm     string
}

// authorize returns ctx carrying the caller id, or the connect error to send.
func (a *rpcAuth) authorize(ctx context.Context, header http.Header) (context.Context, error) {
	h := header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
	}
	claims, err := a.verifier.Verify(strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	ok, err := a.resolver.Can(ctx, claims.UserID, a.perm)
	switch {
	case errors.Is(err, rbac.ErrNoRole):
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("no role assigned"))
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	case !ok:
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("missing permission: "+a.perm))
	}
	return context.WithValue(ctx, ctxUserID, claims.UserID), nil
}

func (a *rpcAuth) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := a.authorize(ctx, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient is a no-op: this interceptor only guards the server.
func (a *rpcAuth) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (a *rpcAuth) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := a.authorize(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// RPCLog writes one line per call, unary or streaming: the procedure, the
// connect code ("ok" on success) and the wall time (a stream's whole life).
// Mount it before RPCAuth so denied calls are logged too; the error text is
// logged only for failures.
func RPCLog(logger *slog.Logger) connect.Interceptor {
	return &rpcLog{logger: logger}
}

type rpcLog struct{ logger *slog.Logger }

func (l *rpcLog) log(ctx context.Context, procedure string, start time.Time, err error) {
	attrs := []any{
		"procedure", procedure,
		"code", "ok",
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		attrs[3] = connect.CodeOf(err).String()
		attrs = append(attrs, "error", err.Error())
	}
	l.logger.InfoContext(ctx, "rpc", attrs...)
}

func (l *rpcLog) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		res, err := next(ctx, req)
		l.log(ctx, req.Spec().Procedure, start, err)
		return res, err
	}
}

// WrapStreamingClient is a no-op: only server calls are logged.
func (l *rpcLog) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (l *rpcLog) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		err := next(ctx, conn)
		l.log(ctx, conn.Spec().Procedure, start, err)
		return err
	}
}
