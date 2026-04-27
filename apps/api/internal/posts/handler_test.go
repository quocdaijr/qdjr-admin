package posts

import (
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

// stubReader is a Reader test double that records the last filter/slug it
// received and returns canned data.
type stubReader struct {
	listOut    []Post
	listTotal  int
	listErr    error
	lastFilter ListFilter

	slugOut  Post
	slugErr  error
	lastSlug string
}

func (s *stubReader) ListPublished(_ context.Context, f ListFilter) ([]Post, int, error) {
	s.lastFilter = f
	return s.listOut, s.listTotal, s.listErr
}

func (s *stubReader) GetPublishedBySlug(_ context.Context, slug string) (Post, error) {
	s.lastSlug = slug
	return s.slugOut, s.slugErr
}

func newRouter(reader Reader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/v1")
	RegisterPublic(g, reader)
	return r
}

func TestList_DefaultsAndEnvelope(t *testing.T) {
	now := time.Now().UTC()
	post := Post{
		ID:          uuid.New(),
		Slug:        "hello",
		Title:       "Hello",
		Content:     "body",
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        []Tag{},
	}
	reader := &stubReader{listOut: []Post{post}, listTotal: 1}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/posts", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data []Post `json:"data"`
		Meta struct {
			Page    int `json:"page"`
			PerPage int `json:"perPage"`
			Total   int `json:"total"`
		} `json:"meta"`
		Error any `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Error)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "hello", body.Data[0].Slug)
	assert.Equal(t, 1, body.Meta.Page)
	assert.Equal(t, 20, body.Meta.PerPage)
	assert.Equal(t, 1, body.Meta.Total)

	assert.Equal(t, 1, reader.lastFilter.Page)
	assert.Equal(t, 20, reader.lastFilter.PerPage)
}

func TestList_ClampsPerPage(t *testing.T) {
	reader := &stubReader{listOut: []Post{}, listTotal: 0}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/posts?page=2&perPage=200", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	assert.Equal(t, 2, reader.lastFilter.Page)
	assert.Equal(t, 100, reader.lastFilter.PerPage, "perPage clamped to 100")
}

func TestList_PassesFilters(t *testing.T) {
	reader := &stubReader{listOut: []Post{}, listTotal: 0}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/posts?category=news&tag=golang&q=launch", nil)
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	assert.Equal(t, "news", reader.lastFilter.CategorySlug)
	assert.Equal(t, "golang", reader.lastFilter.TagSlug)
	assert.Equal(t, "launch", reader.lastFilter.Q)
}

func TestList_NegativePageFallsBackToDefault(t *testing.T) {
	reader := &stubReader{listOut: []Post{}, listTotal: 0}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/posts?page=-3&perPage=abc", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, reader.lastFilter.Page)
	assert.Equal(t, 20, reader.lastFilter.PerPage)
}

func TestSlug_OK(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubReader{slugOut: Post{
		ID: uuid.New(), Slug: "hello", Title: "Hello",
		PublishedAt: &now, CreatedAt: now, UpdatedAt: now, Tags: []Tag{},
	}}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/posts/hello", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data  Post `json:"data"`
		Error any  `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Error)
	assert.Equal(t, "hello", body.Data.Slug)
	assert.Equal(t, "hello", reader.lastSlug)
}

func TestSlug_NotFoundEnvelope(t *testing.T) {
	reader := &stubReader{slugErr: ErrNotFound}
	r := newRouter(reader)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/posts/missing", nil))
	require.Equal(t, http.StatusNotFound, rr.Code)

	var body struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Data)
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
}
