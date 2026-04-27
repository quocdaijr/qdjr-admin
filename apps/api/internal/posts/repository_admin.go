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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminRepository provides admin-side CRUD for posts. It shares the same pool
// as the public Repository but owns its own queries (different shape: includes
// status, all categories, ownership fields).
type AdminRepository struct {
	pool *pgxpool.Pool
}

// NewAdminRepository constructs an AdminRepository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

// adminSelectColumns lists every column exposed in admin responses, including
// JSON aggregates for ALL categories and ALL tags.
const adminSelectColumns = `
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.content,
    p.status::text,
    p.thumbnail_id,
    p.og_image_id,
    p.meta_title,
    p.meta_description,
    p.published_at,
    p.created_by,
    p.updated_by,
    p.created_at,
    p.updated_at,
    COALESCE(
        (SELECT json_agg(json_build_object('id', c.id, 'slug', c.slug, 'name', c.name) ORDER BY c.name)
         FROM public.post_categories pc
         JOIN public.categories c ON c.id = pc.category_id
         WHERE pc.post_id = p.id),
        '[]'::json
    ) AS categories_json,
    COALESCE(
        (SELECT json_agg(json_build_object('id', t.id, 'slug', t.slug, 'name', t.name) ORDER BY t.name)
         FROM public.post_tags pt
         JOIN public.tags t ON t.id = pt.tag_id
         WHERE pt.post_id = p.id),
        '[]'::json
    ) AS tags_json`

// List returns paginated admin posts. Status="" or "all" → no status filter.
func (r *AdminRepository) List(ctx context.Context, f AdminListFilter) ([]AdminPost, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}

	var status *string
	if f.Status != "" && f.Status != "all" {
		s := f.Status
		status = &s
	}
	q := nullableText(strings.TrimSpace(f.Q))

	var (
		createdBy *uuid.UUID
	)
	if f.CreatedBy != nil {
		createdBy = f.CreatedBy
	}

	whereSQL := `
        WHERE ($1::public.post_status IS NULL OR p.status = $1::public.post_status)
          AND ($2::text IS NULL OR p.title ILIKE '%' || $2 || '%')
          AND ($3::uuid IS NULL OR p.created_by = $3::uuid)`

	listSQL := `SELECT ` + adminSelectColumns + `
        FROM public.posts p` + whereSQL + `
        ORDER BY p.created_at DESC, p.id
        LIMIT $4 OFFSET $5`

	offset := (f.Page - 1) * f.PerPage
	rows, err := r.pool.Query(ctx, listSQL, status, q, createdBy, f.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("admin posts list: %w", err)
	}
	defer rows.Close()

	out := make([]AdminPost, 0, f.PerPage)
	for rows.Next() {
		p, err := scanAdminPost(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("admin posts list scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin posts list iterate: %w", err)
	}

	const countSQL = `
        SELECT count(*) FROM public.posts p
        WHERE ($1::public.post_status IS NULL OR p.status = $1::public.post_status)
          AND ($2::text IS NULL OR p.title ILIKE '%' || $2 || '%')
          AND ($3::uuid IS NULL OR p.created_by = $3::uuid)`

	var total int
	if err := r.pool.QueryRow(ctx, countSQL, status, q, createdBy).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin posts count: %w", err)
	}
	return out, total, nil
}

// GetByID returns a single admin post. Returns ErrNotFound when absent.
func (r *AdminRepository) GetByID(ctx context.Context, id uuid.UUID) (AdminPost, error) {
	const q = `SELECT ` + adminSelectColumns + ` FROM public.posts p WHERE p.id = $1 LIMIT 1`
	row := r.pool.QueryRow(ctx, q, id)
	p, err := scanAdminPost(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminPost{}, ErrNotFound
	}
	if err != nil {
		return AdminPost{}, fmt.Errorf("admin posts get: %w", err)
	}
	return p, nil
}

// Create inserts a post + pivot rows in a single transaction. Returns the
// freshly hydrated AdminPost.
func (r *AdminRepository) Create(ctx context.Context, in CreateInput) (AdminPost, error) {
	id := uuid.New()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AdminPost{}, fmt.Errorf("admin posts create begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var publishedAt *time.Time
	if in.Status == "published" {
		now := time.Now().UTC()
		publishedAt = &now
	}

	const insert = `
        INSERT INTO public.posts (
            id, slug, title, excerpt, content, status, thumbnail_id, og_image_id,
            meta_title, meta_description, published_at, created_by, updated_by
        ) VALUES (
            $1, $2, $3, $4, $5, $6::public.post_status, $7, $8, $9, $10, $11, $12, $12
        )`
	if _, err := tx.Exec(ctx, insert,
		id, in.Slug, in.Title, in.Excerpt, in.Content, in.Status,
		in.ThumbnailID, in.OGImageID, in.MetaTitle, in.MetaDescription,
		publishedAt, in.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			return AdminPost{}, ErrSlugConflict
		}
		return AdminPost{}, fmt.Errorf("admin posts insert: %w", err)
	}

	if err := replacePivots(ctx, tx, id, in.CategoryIDs, in.TagIDs, true, true); err != nil {
		return AdminPost{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AdminPost{}, fmt.Errorf("admin posts create commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Update applies a partial update + optional pivot replacement in a transaction.
func (r *AdminRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (AdminPost, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AdminPost{}, fmt.Errorf("admin posts update begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Fetch current row for status-transition logic and existence check.
	const cur = `SELECT status::text, published_at FROM public.posts WHERE id = $1 FOR UPDATE`
	var (
		curStatus      string
		curPublishedAt *time.Time
	)
	if err := tx.QueryRow(ctx, cur, id).Scan(&curStatus, &curPublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminPost{}, ErrNotFound
		}
		return AdminPost{}, fmt.Errorf("admin posts update load: %w", err)
	}

	sets := make([]string, 0, 12)
	args := make([]any, 0, 12)
	nextArg := 1
	add := func(col, cast string, v any) {
		if cast != "" {
			sets = append(sets, fmt.Sprintf("%s = $%d::%s", col, nextArg, cast))
		} else {
			sets = append(sets, fmt.Sprintf("%s = $%d", col, nextArg))
		}
		args = append(args, v)
		nextArg++
	}

	if in.Title != nil {
		add("title", "", *in.Title)
	}
	if in.Slug != nil {
		add("slug", "", *in.Slug)
	}
	if in.Excerpt != nil {
		add("excerpt", "", *in.Excerpt)
	}
	if in.Content != nil {
		add("content", "", *in.Content)
	}
	if in.ThumbnailID != nil {
		add("thumbnail_id", "", *in.ThumbnailID)
	}
	if in.OGImageID != nil {
		add("og_image_id", "", *in.OGImageID)
	}
	if in.MetaTitle != nil {
		add("meta_title", "", *in.MetaTitle)
	}
	if in.MetaDescription != nil {
		add("meta_description", "", *in.MetaDescription)
	}
	if in.Status != nil {
		add("status", "public.post_status", *in.Status)
		// Status transitions: only set published_at when going non-published →
		// published AND there is no existing published_at (preserves first
		// publication timestamp on re-publish).
		if *in.Status == "published" && curStatus != "published" && curPublishedAt == nil {
			now := time.Now().UTC()
			add("published_at", "", now)
		}
	}
	// updated_by + updated_at always change on any field update.
	add("updated_by", "", in.UpdatedBy)
	sets = append(sets, "updated_at = now()")

	if len(sets) > 0 {
		stmt := fmt.Sprintf(`UPDATE public.posts SET %s WHERE id = $%d`,
			strings.Join(sets, ", "), nextArg)
		args = append(args, id)
		if _, err := tx.Exec(ctx, stmt, args...); err != nil {
			if isUniqueViolation(err) {
				return AdminPost{}, ErrSlugConflict
			}
			return AdminPost{}, fmt.Errorf("admin posts update: %w", err)
		}
	}

	// Pivot replacement (only when caller explicitly passed the slice).
	var (
		cats     []uuid.UUID
		tags     []uuid.UUID
		repCats  bool
		repTags  bool
	)
	if in.CategoryIDs != nil {
		cats = *in.CategoryIDs
		repCats = true
	}
	if in.TagIDs != nil {
		tags = *in.TagIDs
		repTags = true
	}
	if err := replacePivots(ctx, tx, id, cats, tags, repCats, repTags); err != nil {
		return AdminPost{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AdminPost{}, fmt.Errorf("admin posts update commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Delete removes a post (cascades pivots via FK ON DELETE CASCADE).
func (r *AdminRepository) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM public.posts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("admin posts delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPublished publishes (publish=true) or unpublishes (publish=false) a post.
// Idempotent: re-publishing keeps the original published_at; unpublishing
// flips status to draft and nulls published_at.
func (r *AdminRepository) SetPublished(ctx context.Context, id uuid.UUID, publish bool, updatedBy uuid.UUID) (AdminPost, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AdminPost{}, fmt.Errorf("admin posts publish begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const cur = `SELECT status::text, published_at FROM public.posts WHERE id = $1 FOR UPDATE`
	var (
		curStatus      string
		curPublishedAt *time.Time
	)
	if err := tx.QueryRow(ctx, cur, id).Scan(&curStatus, &curPublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminPost{}, ErrNotFound
		}
		return AdminPost{}, fmt.Errorf("admin posts publish load: %w", err)
	}

	if publish {
		// Set published_at only on first publish.
		var newPublished any = curPublishedAt
		if curPublishedAt == nil {
			t := time.Now().UTC()
			newPublished = t
		}
		_, err = tx.Exec(ctx,
			`UPDATE public.posts SET status='published'::public.post_status, published_at=$1, updated_by=$2, updated_at=now() WHERE id=$3`,
			newPublished, updatedBy, id)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE public.posts SET status='draft'::public.post_status, published_at=NULL, updated_by=$1, updated_at=now() WHERE id=$2`,
			updatedBy, id)
	}
	if err != nil {
		return AdminPost{}, fmt.Errorf("admin posts publish update: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminPost{}, fmt.Errorf("admin posts publish commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

// replacePivots clears post_categories / post_tags for postID and reinserts
// the provided IDs. When repCats/repTags is false, the corresponding pivot is
// left untouched (used by Create where both are always replaced from empty).
func replacePivots(ctx context.Context, tx pgx.Tx, postID uuid.UUID, cats, tags []uuid.UUID, repCats, repTags bool) error {
	if repCats {
		if _, err := tx.Exec(ctx, `DELETE FROM public.post_categories WHERE post_id = $1`, postID); err != nil {
			return fmt.Errorf("delete post_categories: %w", err)
		}
		for _, cid := range cats {
			if _, err := tx.Exec(ctx,
				`INSERT INTO public.post_categories (post_id, category_id) VALUES ($1, $2)`,
				postID, cid); err != nil {
				return fmt.Errorf("insert post_categories: %w", err)
			}
		}
	}
	if repTags {
		if _, err := tx.Exec(ctx, `DELETE FROM public.post_tags WHERE post_id = $1`, postID); err != nil {
			return fmt.Errorf("delete post_tags: %w", err)
		}
		for _, tid := range tags {
			if _, err := tx.Exec(ctx,
				`INSERT INTO public.post_tags (post_id, tag_id) VALUES ($1, $2)`,
				postID, tid); err != nil {
				return fmt.Errorf("insert post_tags: %w", err)
			}
		}
	}
	return nil
}

// scanAdminPost decodes one admin row.
func scanAdminPost(row rowScanner) (AdminPost, error) {
	var (
		p          AdminPost
		excerpt    *string
		mTitle     *string
		mDesc      *string
		catsJSON   []byte
		tagsJSON   []byte
	)
	if err := row.Scan(
		&p.ID, &p.Slug, &p.Title, &excerpt, &p.Content, &p.Status,
		&p.ThumbnailID, &p.OGImageID, &mTitle, &mDesc,
		&p.PublishedAt, &p.CreatedBy, &p.UpdatedBy,
		&p.CreatedAt, &p.UpdatedAt,
		&catsJSON, &tagsJSON,
	); err != nil {
		return AdminPost{}, err
	}
	p.Excerpt = excerpt
	p.MetaTitle = mTitle
	p.MetaDescription = mDesc
	p.Categories = []Category{}
	p.Tags = []Tag{}
	if len(catsJSON) > 0 {
		var cats []Category
		if err := json.Unmarshal(catsJSON, &cats); err != nil {
			return AdminPost{}, fmt.Errorf("decode admin categories: %w", err)
		}
		if cats != nil {
			p.Categories = cats
		}
	}
	if len(tagsJSON) > 0 {
		var tags []Tag
		if err := json.Unmarshal(tagsJSON, &tags); err != nil {
			return AdminPost{}, fmt.Errorf("decode admin tags: %w", err)
		}
		if tags != nil {
			p.Tags = tags
		}
	}
	return p, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
