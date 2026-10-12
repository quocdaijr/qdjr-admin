package media

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type s3Call struct {
	method, path, contentType, body string
}

func fakeS3(t *testing.T, status int) (*httptest.Server, *[]s3Call) {
	t.Helper()
	var mu sync.Mutex
	calls := []s3Call{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, s3Call{r.Method, r.URL.Path, r.Header.Get("Content-Type"), string(b)})
		mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestR2_PutAndDelete(t *testing.T) {
	srv, calls := fakeS3(t, http.StatusOK)
	st := newR2WithEndpoint(srv.URL, "key-id", "secret", "qdjr-media")

	require.NoError(t, st.Put(context.Background(), "media/a.png", "image/png", strings.NewReader("PNGDATA"), 7))
	require.NoError(t, st.Delete(context.Background(), "media/a.png"))

	require.Len(t, *calls, 2)
	put := (*calls)[0]
	assert.Equal(t, http.MethodPut, put.method)
	assert.Equal(t, "/qdjr-media/media/a.png", put.path, "path-style bucket addressing")
	assert.Equal(t, "image/png", put.contentType)
	assert.Contains(t, put.body, "PNGDATA")
	assert.Equal(t, http.MethodDelete, (*calls)[1].method)
}

func TestR2_PutErrorSurfaces(t *testing.T) {
	srv, _ := fakeS3(t, http.StatusForbidden)
	st := newR2WithEndpoint(srv.URL, "key-id", "secret", "qdjr-media")
	err := st.Put(context.Background(), "media/a.png", "image/png", strings.NewReader("x"), 1)
	assert.Error(t, err)
}

func TestNewR2_RequiresAllCredentials(t *testing.T) {
	assert.Nil(t, NewR2("acct", "", "secret", "bucket"))
	assert.NotNil(t, NewR2("acct", "id", "secret", "bucket"))
}

func TestLocalStorage_PutDelete(t *testing.T) {
	dir := t.TempDir()
	st := NewLocalStorage(dir)
	require.NoError(t, st.Put(context.Background(), "media/b.png", "image/png", strings.NewReader("hello"), 5))
	got, err := os.ReadFile(filepath.Join(dir, "media", "b.png"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))

	require.NoError(t, st.Delete(context.Background(), "media/b.png"))
	_, err = os.Stat(filepath.Join(dir, "media", "b.png"))
	assert.True(t, os.IsNotExist(err))
	assert.NoError(t, st.Delete(context.Background(), "media/b.png"), "missing file is not an error")
}

func TestLocalStorage_RejectsPathEscape(t *testing.T) {
	st := NewLocalStorage(t.TempDir())
	assert.Error(t, st.Put(context.Background(), "../escape.png", "image/png", strings.NewReader("x"), 1))
	assert.Error(t, st.Delete(context.Background(), "../../etc/passwd"))
}
