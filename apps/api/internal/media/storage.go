package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StorageClient is a small Supabase Storage admin client backed by net/http.
// It uses the service-role key for authorization and supports the operations
// the admin media handler needs: ensure-bucket, signed-upload-url, delete.
type StorageClient struct {
	baseURL        string // e.g. "http://127.0.0.1:54321"
	serviceRoleKey string
	bucket         string
	hc             *http.Client
}

// NewStorageClient constructs a StorageClient. baseURL should NOT include the
// trailing "/storage/v1" — that path is added internally.
func NewStorageClient(baseURL, serviceRoleKey, bucket string) *StorageClient {
	return &StorageClient{
		baseURL:        strings.TrimRight(baseURL, "/"),
		serviceRoleKey: serviceRoleKey,
		bucket:         bucket,
		hc:             &http.Client{Timeout: 15 * time.Second},
	}
}

// Bucket returns the bucket name configured at construction.
func (s *StorageClient) Bucket() string { return s.bucket }

// EnsureBucket creates the configured bucket as public. The call is idempotent
// from the caller's perspective: an existing bucket (Supabase returns HTTP 400
// with embedded statusCode "409") is treated as success.
func (s *StorageClient) EnsureBucket(ctx context.Context) error {
	body := map[string]any{"id": s.bucket, "name": s.bucket, "public": true}
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("media storage: marshal bucket body: %w", err)
	}
	url := s.baseURL + "/storage/v1/bucket"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("media storage: build bucket request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.hc.Do(req)
	if err != nil {
		return fmt.Errorf("media storage: bucket request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// Treat duplicate-bucket as success (Supabase returns 400 with statusCode "409").
	if isDuplicateError(respBody) {
		return nil
	}
	return fmt.Errorf("media storage: ensure bucket: status=%d body=%s",
		resp.StatusCode, string(respBody))
}

// SignedUploadURL requests a signed-URL for uploading to bucket/path. It
// returns the absolute URL the client should PUT/POST the file to.
func (s *StorageClient) SignedUploadURL(ctx context.Context, path string) (string, error) {
	endpoint := fmt.Sprintf("%s/storage/v1/object/upload/sign/%s/%s",
		s.baseURL, s.bucket, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("media storage: build sign request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)

	resp, err := s.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("media storage: sign request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("media storage: sign url: status=%d body=%s",
			resp.StatusCode, string(respBody))
	}
	var parsed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("media storage: decode sign response: %w", err)
	}
	if parsed.URL == "" {
		return "", fmt.Errorf("media storage: sign response missing url: %s", string(respBody))
	}
	// Supabase returns a path like "/object/upload/sign/<bucket>/<path>?token=..."
	// → expand to absolute URL the client can PUT to.
	if strings.HasPrefix(parsed.URL, "/") {
		return s.baseURL + "/storage/v1" + parsed.URL, nil
	}
	return parsed.URL, nil
}

// Delete removes an object at bucket/path. Best-effort: a 404 from storage
// is not surfaced as an error so DB cleanup is unblocked.
func (s *StorageClient) Delete(ctx context.Context, path string) error {
	endpoint := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, s.bucket, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("media storage: build delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)

	resp, err := s.hc.Do(req)
	if err != nil {
		return fmt.Errorf("media storage: delete request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if isNotFoundError(resp.StatusCode, respBody) {
		return nil
	}
	return fmt.Errorf("media storage: delete: status=%d body=%s",
		resp.StatusCode, string(respBody))
}

// isDuplicateError reports whether a Supabase Storage error body carries a 409
// duplicate marker.
func isDuplicateError(body []byte) bool {
	var parsed struct {
		StatusCode string `json:"statusCode"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	if parsed.StatusCode == "409" {
		return true
	}
	return strings.EqualFold(parsed.Error, "Duplicate")
}

// isNotFoundError reports whether a storage delete response indicates the
// object was missing (treated as success for idempotent delete).
func isNotFoundError(status int, body []byte) bool {
	if status == http.StatusNotFound {
		return true
	}
	var parsed struct {
		StatusCode string `json:"statusCode"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	return parsed.StatusCode == "404" || strings.EqualFold(parsed.Error, "not_found")
}
