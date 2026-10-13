package http

import (
	"net/http"
	"net/http/pprof"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpcreflect/v2"
)

// MountDevTools adds developer endpoints to the outer mux, outside production
// only:
//   - gRPC server reflection (v1 and v1alpha) for services, so grpcurl can
//     list and describe them without -proto;
//   - net/http/pprof under /debug/pprof/ (index, named profiles, CPU profile,
//     trace), for go tool pprof.
//
// Both publish internals with no auth (the schema; heap and goroutine
// stacks), which is why production never gets them; there their paths fall
// through to Gin's 404.
func MountDevTools(mux *http.ServeMux, env string, services ...string) {
	if env == "production" {
		return
	}
	// Reflection gets its own server so the lessons server's auth does not
	// apply: grpcurl list/describe work without a token, in dev only.
	reflection := connect.NewServer()
	grpcreflect.Register(reflection, grpcreflect.WithNamer(grpcreflect.NamerFunc(func() []string { return services })))
	connecthttp.Mount(mux, reflection)

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
