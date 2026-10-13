package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/grpcreflect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Registers lessons.v1 in the global proto registry the reflector reads.
	_ "github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1"
)

const lessonsService = "lessons.v1.LessonsService"

// devServer is the outer mux shape from cmd/api: dev tools, then a catch-all
// standing in for Gin. h2c on, because reflection is a bidi stream.
func devServer(t *testing.T, env string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	MountDevTools(mux, env, lessonsService)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	srv := httptest.NewUnstartedServer(mux)
	var p http.Protocols
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = &p
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func h2cClient() *http.Client {
	var p http.Protocols
	p.SetUnencryptedHTTP2(true)
	return &http.Client{Transport: &http.Transport{Protocols: &p}}
}

func TestMountDevTools_Development_ReflectionListsServices(t *testing.T) {
	srv := devServer(t, "development")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := grpcreflect.NewClient(h2cClient(), srv.URL).NewStream(ctx)
	names, err := stream.ListServices()
	require.NoError(t, err)
	_, _ = stream.Close()
	var got []string
	for _, n := range names {
		got = append(got, string(n))
	}
	assert.Contains(t, got, lessonsService)
}

// TestMountDevTools_Production_NoReflection pins that reflection (schema
// disclosure, no auth) never ships: the path falls through to the catch-all.
func TestMountDevTools_Production_NoReflection(t *testing.T) {
	srv := devServer(t, "production")
	for _, path := range []string{
		"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	} {
		req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/grpc")
		res, err := h2cClient().Do(req)
		require.NoError(t, err)
		_ = res.Body.Close()
		assert.Equal(t, http.StatusNotFound, res.StatusCode, path)
	}
}

func TestMountDevTools_Development_Pprof(t *testing.T) {
	srv := devServer(t, "development")
	res, err := http.Get(srv.URL + "/debug/pprof/")
	require.NoError(t, err)
	_ = res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

// TestMountDevTools_Production_NoPprof pins that profiling endpoints (heap,
// goroutine stacks, CPU on demand) never ship to production.
func TestMountDevTools_Production_NoPprof(t *testing.T) {
	srv := devServer(t, "production")
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/debug/pprof/profile?seconds=1"} {
		res, err := http.Get(srv.URL + path)
		require.NoError(t, err)
		_ = res.Body.Close()
		assert.Equal(t, http.StatusNotFound, res.StatusCode, path)
	}
}
