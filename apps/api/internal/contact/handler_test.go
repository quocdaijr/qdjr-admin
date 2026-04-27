package contact

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubWriter is a Writer test double.
type stubWriter struct {
	out   Created
	err   error
	calls []CreateInput
}

func (s *stubWriter) Create(_ context.Context, in CreateInput) (Created, error) {
	s.calls = append(s.calls, in)
	return s.out, s.err
}

func newRouter(repo Writer, l *Limiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/v1")
	RegisterPublic(g, repo, l)
	return r
}

func postJSON(t *testing.T, r http.Handler, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/contact", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestPost_Created(t *testing.T) {
	now := time.Now().UTC()
	id := uuid.New()
	repo := &stubWriter{out: Created{ID: id, CreatedAt: now}}
	r := newRouter(repo, NewLimiter())

	rr := postJSON(t, r, map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"body":  "hello!",
	}, map[string]string{"X-Forwarded-For": "9.9.9.9"})
	require.Equal(t, http.StatusCreated, rr.Code)

	var body struct {
		Data struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
		} `json:"data"`
		Error any `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Error)
	assert.Equal(t, id.String(), body.Data.ID)

	require.Len(t, repo.calls, 1)
	assert.Equal(t, "Alice", repo.calls[0].Name)
	assert.Equal(t, "9.9.9.9", repo.calls[0].IP)
}

func TestPost_ValidationMissingFields(t *testing.T) {
	repo := &stubWriter{}
	r := newRouter(repo, NewLimiter())

	rr := postJSON(t, r, map[string]any{
		"name":  "",
		"email": "alice@example.com",
		"body":  "hi",
	}, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)

	var body struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "VALIDATION", body.Error.Code)
	assert.Contains(t, body.Error.Message, "name")
	assert.Empty(t, repo.calls, "repo must not be called on invalid input")
}

func TestPost_ValidationInvalidEmail(t *testing.T) {
	repo := &stubWriter{}
	r := newRouter(repo, NewLimiter())

	rr := postJSON(t, r, map[string]any{
		"name":  "Alice",
		"email": "not-an-email",
		"body":  "hi",
	}, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Empty(t, repo.calls)
}

func TestPost_BadJSON(t *testing.T) {
	repo := &stubWriter{}
	r := newRouter(repo, NewLimiter())

	req := httptest.NewRequest(http.MethodPost, "/v1/contact", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Empty(t, repo.calls)
}

func TestPost_RateLimited(t *testing.T) {
	repo := &stubWriter{out: Created{ID: uuid.New(), CreatedAt: time.Now()}}
	limiter := NewLimiter()
	r := newRouter(repo, limiter)

	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"body":  "hello",
	}
	headers := map[string]string{"X-Forwarded-For": "5.5.5.5"}

	// Burn the burst.
	for i := 0; i < limiterBurst; i++ {
		rr := postJSON(t, r, body, headers)
		require.Equalf(t, http.StatusCreated, rr.Code, "request %d should succeed", i+1)
	}

	rr := postJSON(t, r, body, headers)
	require.Equal(t, http.StatusTooManyRequests, rr.Code)

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "RATE_LIMITED", resp.Error.Code)
}

func TestPost_RepoError(t *testing.T) {
	repo := &stubWriter{err: errStr("db down")}
	r := newRouter(repo, NewLimiter())

	rr := postJSON(t, r, map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"body":  "hi",
	}, nil)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestClientIP_Headers(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		remote  string
		want    string
	}{
		{
			name:    "X-Forwarded-For first hop",
			headers: map[string]string{"X-Forwarded-For": "1.1.1.1, 2.2.2.2"},
			remote:  "127.0.0.1:1234",
			want:    "1.1.1.1",
		},
		{
			name:    "X-Real-IP fallback",
			headers: map[string]string{"X-Real-IP": "3.3.3.3"},
			remote:  "127.0.0.1:1234",
			want:    "3.3.3.3",
		},
		{
			name:   "RemoteAddr fallback",
			remote: "4.4.4.4:5678",
			want:   "4.4.4.4",
		},
	}
	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodPost, "/v1/contact", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tc.remote
			c.Request = req
			assert.Equal(t, tc.want, clientIP(c))
		})
	}
}

type errStr string

func (e errStr) Error() string { return string(e) }
