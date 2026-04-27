package categories

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

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
)

// stubReader is a Reader test double for the categories repository.
type stubReader struct {
	listOut []Resource
	listErr error

	slugOut  Resource
	slugErr  error
	lastSlug string
}

func (s *stubReader) List(_ context.Context) ([]Resource, error) {
	return s.listOut, s.listErr
}

func (s *stubReader) GetBySlug(_ context.Context, slug string) (Resource, error) {
	s.lastSlug = slug
	return s.slugOut, s.slugErr
}

// stubPostsReader is a posts.Reader test double.
type stubPostsReader struct {
	listOut    []posts.Post
	listTotal  int
	listErr    error
	lastFilter posts.ListFilter
}

func (s *stubPostsReader) ListPublished(_ context.Context, f posts.ListFilter) ([]posts.Post, int, error) {
	s.lastFilter = f
	return s.listOut, s.listTotal, s.listErr
}

func (s *stubPostsReader) GetPublishedBySlug(_ context.Context, _ string) (posts.Post, error) {
	return posts.Post{}, posts.ErrNotFound
}

func newRouter(reader Reader, postsReader posts.Reader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/v1")
	RegisterPublic(g, reader, postsReader)
	return r
}

func TestList_OK(t *testing.T) {
	desc := "news category"
	now := time.Now().UTC()
	res := Resource{
		ID:          uuid.New(),
		Slug:        "news",
		Name:        "News",
		Description: &desc,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	reader := &stubReader{listOut: []Resource{res}}
	r := newRouter(reader, &stubPostsReader{})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/categories", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data  []Resource `json:"data"`
		Error any        `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Nil(t, body.Error)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "news", body.Data[0].Slug)
	require.NotNil(t, body.Data[0].Description)
	assert.Equal(t, "news category", *body.Data[0].Description)
}

func TestList_EmptyArray(t *testing.T) {
	reader := &stubReader{listOut: []Resource{}}
	r := newRouter(reader, &stubPostsReader{})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/categories", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	// Ensure we serialize as [] not null.
	assert.Contains(t, rr.Body.String(), `"data":[]`)
}

func TestPostsBySlug_OK(t *testing.T) {
	now := time.Now().UTC()
	cat := Resource{ID: uuid.New(), Slug: "news", Name: "News", CreatedAt: now, UpdatedAt: now}
	reader := &stubReader{slugOut: cat}

	post := posts.Post{
		ID: uuid.New(), Slug: "hello", Title: "Hello",
		PublishedAt: &now, CreatedAt: now, UpdatedAt: now, Tags: []posts.Tag{},
	}
	pr := &stubPostsReader{listOut: []posts.Post{post}, listTotal: 1}
	r := newRouter(reader, pr)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/categories/news/posts?page=2&perPage=200&q=foo", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data []posts.Post `json:"data"`
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
	assert.Equal(t, 2, body.Meta.Page)
	assert.Equal(t, 100, body.Meta.PerPage, "perPage clamped to 100")
	assert.Equal(t, 1, body.Meta.Total)

	assert.Equal(t, "news", reader.lastSlug)
	assert.Equal(t, "news", pr.lastFilter.CategorySlug)
	assert.Equal(t, "foo", pr.lastFilter.Q)
	assert.Equal(t, 2, pr.lastFilter.Page)
	assert.Equal(t, 100, pr.lastFilter.PerPage)
}

func TestPostsBySlug_NotFoundEnvelope(t *testing.T) {
	reader := &stubReader{slugErr: ErrNotFound}
	pr := &stubPostsReader{}
	r := newRouter(reader, pr)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/categories/missing/posts", nil))
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
	// posts reader must not be called when category doesn't exist.
	assert.Equal(t, posts.ListFilter{}, pr.lastFilter)
}

func TestPostsBySlug_EmptyButExisting(t *testing.T) {
	now := time.Now().UTC()
	cat := Resource{ID: uuid.New(), Slug: "news", Name: "News", CreatedAt: now, UpdatedAt: now}
	reader := &stubReader{slugOut: cat}
	pr := &stubPostsReader{listOut: []posts.Post{}, listTotal: 0}
	r := newRouter(reader, pr)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/categories/news/posts", nil))
	require.Equal(t, http.StatusOK, rr.Code, "existing slug with zero posts must be 200, not 404")
	assert.Contains(t, rr.Body.String(), `"data":[]`)
}
