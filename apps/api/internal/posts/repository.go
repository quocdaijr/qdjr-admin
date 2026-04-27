package posts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads published posts from Postgres.
//
// urlPrefix is prepended to media.storage_path to produce a public URL,
// e.g. "https://<project>.supabase.co/storage/v1/object/public/". When empty
// the raw storage_path is returned (test convenience; FE may prefix itself).
type Repository struct {
	pool      *pgxpool.Pool
	urlPrefix string
}

// NewRepository constructs a Repository. Pass the Supabase storage public-URL
// prefix as urlPrefix (e.g. cfg.SupabaseURL + "/storage/v1/object/public/") so
// the API returns directly-loadable URLs. May be empty in tests.
func NewRepository(pool *pgxpool.Pool, urlPrefix string) *Repository {
	return &Repository{pool: pool, urlPrefix: urlPrefix}
}

// selectColumns is the canonical column list shared by list + slug queries.
// It uses a LATERAL subquery to pick a single category per post (avoids the
// duplicate-row problem when a post belongs to multiple categories) and a
// scalar subquery for tags as JSON to keep scanning simple.
const selectColumns = `
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.content,
    p.published_at,
    p.created_at,
    p.updated_at,
    m.id           AS thumb_id,
    m.storage_path AS thumb_path,
    m.alt_text     AS thumb_alt,
    c.id           AS cat_id,
    c.slug         AS cat_slug,
    c.name         AS cat_name,
    COALESCE(
        (SELECT json_agg(json_build_object('id', t.id, 'slug', t.slug, 'name', t.name) ORDER BY t.name)
         FROM public.post_tags pt
         JOIN public.tags t ON t.id = pt.tag_id
         WHERE pt.post_id = p.id),
        '[]'::json
    ) AS tags_json`

const fromClause = `
    FROM public.posts p
    LEFT JOIN public.media m ON m.id = p.thumbnail_id
    LEFT JOIN LATERAL (
        SELECT c.id, c.slug, c.name
        FROM public.post_categories pc
        JOIN public.categories c ON c.id = pc.category_id
        WHERE pc.post_id = p.id
        ORDER BY c.name
        LIMIT 1
    ) c ON true`

// ListPublished returns published posts (newest first) and the total count
// matching the same filter (ignoring pagination).
func (r *Repository) ListPublished(ctx context.Context, f ListFilter) ([]Post, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}

	var (
		category = nullableText(f.CategorySlug)
		tag      = nullableText(f.TagSlug)
		q        = nullableText(strings.TrimSpace(f.Q))
	)

	whereSQL := `
        WHERE p.status = 'published'
          AND ($1::text IS NULL OR EXISTS (
                SELECT 1 FROM public.post_categories pc2
                JOIN public.categories c2 ON c2.id = pc2.category_id
                WHERE pc2.post_id = p.id AND c2.slug = $1))
          AND ($2::text IS NULL OR EXISTS (
                SELECT 1 FROM public.post_tags pt2
                JOIN public.tags t2 ON t2.id = pt2.tag_id
                WHERE pt2.post_id = p.id AND t2.slug = $2))
          AND ($3::text IS NULL OR p.title ILIKE '%' || $3 || '%')`

	listSQL := `SELECT ` + selectColumns + fromClause + whereSQL + `
        ORDER BY p.published_at DESC NULLS LAST, p.id
        LIMIT $4 OFFSET $5`

	offset := (f.Page - 1) * f.PerPage
	rows, err := r.pool.Query(ctx, listSQL, category, tag, q, f.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("posts list: %w", err)
	}
	defer rows.Close()

	out := make([]Post, 0, f.PerPage)
	for rows.Next() {
		p, err := r.scanPost(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("posts list scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("posts list iterate: %w", err)
	}

	const countSQL = `
        SELECT count(*) FROM public.posts p
        WHERE p.status = 'published'
          AND ($1::text IS NULL OR EXISTS (
                SELECT 1 FROM public.post_categories pc2
                JOIN public.categories c2 ON c2.id = pc2.category_id
                WHERE pc2.post_id = p.id AND c2.slug = $1))
          AND ($2::text IS NULL OR EXISTS (
                SELECT 1 FROM public.post_tags pt2
                JOIN public.tags t2 ON t2.id = pt2.tag_id
                WHERE pt2.post_id = p.id AND t2.slug = $2))
          AND ($3::text IS NULL OR p.title ILIKE '%' || $3 || '%')`

	var total int
	if err := r.pool.QueryRow(ctx, countSQL, category, tag, q).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("posts count: %w", err)
	}
	return out, total, nil
}

// GetPublishedBySlug returns a single published post or ErrNotFound.
func (r *Repository) GetPublishedBySlug(ctx context.Context, slug string) (Post, error) {
	const slugSQL = `SELECT ` + selectColumns + fromClause + `
        WHERE p.status = 'published' AND p.slug = $1
        LIMIT 1`
	row := r.pool.QueryRow(ctx, slugSQL, slug)
	p, err := r.scanPost(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("posts slug: %w", err)
	}
	return p, nil
}

// rowScanner abstracts pgx.Row and pgx.Rows for scanPost.
type rowScanner interface {
	Scan(dest ...any) error
}

func (r *Repository) scanPost(row rowScanner) (Post, error) {
	var (
		p           Post
		excerpt     *string
		publishedAt *time.Time
		createdAt   time.Time
		updatedAt   time.Time

		thumbID   *uuid.UUID
		thumbPath *string
		thumbAlt  *string

		catID   *uuid.UUID
		catSlug *string
		catName *string

		tagsJSON []byte
	)
	if err := row.Scan(
		&p.ID, &p.Slug, &p.Title, &excerpt, &p.Content, &publishedAt, &createdAt, &updatedAt,
		&thumbID, &thumbPath, &thumbAlt,
		&catID, &catSlug, &catName,
		&tagsJSON,
	); err != nil {
		return Post{}, err
	}

	p.Excerpt = excerpt
	p.PublishedAt = publishedAt
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	p.Location = nil
	p.Tags = []Tag{}

	if thumbID != nil && thumbPath != nil {
		t := Thumbnail{URL: r.thumbnailURL(*thumbPath)}
		if thumbAlt != nil {
			t.Alt = *thumbAlt
		}
		p.Thumbnail = &t
	}

	if catID != nil && catSlug != nil && catName != nil {
		p.Category = &Category{ID: *catID, Slug: *catSlug, Name: *catName}
	}

	if len(tagsJSON) > 0 {
		var tags []Tag
		if err := json.Unmarshal(tagsJSON, &tags); err != nil {
			return Post{}, fmt.Errorf("decode tags: %w", err)
		}
		if tags != nil {
			p.Tags = tags
		}
	}
	return p, nil
}

func (r *Repository) thumbnailURL(storagePath string) string {
	if r.urlPrefix == "" {
		return storagePath
	}
	if strings.HasSuffix(r.urlPrefix, "/") {
		return r.urlPrefix + storagePath
	}
	return r.urlPrefix + "/" + storagePath
}

// nullableText returns *string=nil for empty, otherwise a pointer to v. pgx
// binds *string=nil as SQL NULL, which we test against in the WHERE clauses.
func nullableText(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
