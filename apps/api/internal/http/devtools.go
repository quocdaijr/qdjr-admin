package http

import (
	"net/http"

	"connectrpc.com/grpcreflect"
)

// MountDevTools adds developer endpoints to the outer mux, outside production
// only. Today that is gRPC server reflection (v1 and v1alpha) for services,
// so grpcurl can list and describe them without -proto. Reflection publishes
// the whole schema and has no auth, which is why production never gets it;
// there its paths fall through to Gin's 404.
func MountDevTools(mux *http.ServeMux, env string, services ...string) {
	if env == "production" {
		return
	}
	reflector := grpcreflect.NewStaticReflector(services...)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
}
