package media

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStorageFromEnv(t *testing.T) *StorageClient {
	t.Helper()
	url := os.Getenv("TEST_SUPABASE_URL")
	key := os.Getenv("TEST_SUPABASE_SERVICE_ROLE_KEY")
	if url == "" || key == "" {
		t.Skip("set TEST_SUPABASE_URL and TEST_SUPABASE_SERVICE_ROLE_KEY")
	}
	return NewStorageClient(url, key, "media")
}

func TestStorageClient_EnsureBucket_Idempotent(t *testing.T) {
	s := newStorageFromEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, s.EnsureBucket(ctx))
	// Second call must succeed even though bucket already exists.
	require.NoError(t, s.EnsureBucket(ctx))
}

func TestStorageClient_Delete_MissingTreatedAsSuccess(t *testing.T) {
	s := newStorageFromEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, s.EnsureBucket(ctx))

	// Deleting a non-existent object must not error (idempotent).
	err := s.Delete(ctx, "media/"+uuid.New().String()+".png")
	require.NoError(t, err)
}

func TestStorageClient_Bucket(t *testing.T) {
	s := NewStorageClient("http://localhost", "k", "media-x")
	assert.Equal(t, "media-x", s.Bucket())
}

func TestStorageClient_SignedUploadURL(t *testing.T) {
	s := newStorageFromEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, s.EnsureBucket(ctx))

	path := "media/" + uuid.New().String() + ".png"
	url, err := s.SignedUploadURL(ctx, path)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.True(t, strings.Contains(url, "token="), "signed url must carry a token: %s", url)
	assert.True(t, strings.Contains(url, path), "signed url must reference the upload path: %s", url)
}
