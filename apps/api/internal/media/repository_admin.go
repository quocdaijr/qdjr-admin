package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminRepository provides admin list/get/create/delete for media. URLs are
// computed from storage_path using the public storage prefix supplied at
// construction.
type AdminRepository struct {
	pool       *pgxpool.Pool
	urlPrefix  string
}

// NewAdminRepository constructs an AdminRepository. urlPrefix should be the
// Supabase public-object base, e.g. "http://.../storage/v1/object/public/".
func NewAdminRepository(pool *pgxpool.Pool, urlPrefix string) *AdminRepository {
	return &AdminRepository{pool: pool, urlPrefix: urlPrefix}
}

const adminSelectColumns = `
    id, filename, storage_path, mime_type, size,
    width, height, alt_text, uploaded_by, created_at`

// List returns paginated media rows ordered by created_at DESC.
func (r *AdminRepository) List(ctx context.Context, f AdminListFilter) ([]Media, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}

	const listSQL = `
        SELECT ` + adminSelectColumns + `
        FROM public.media
        ORDER BY created_at DESC, id
        LIMIT $1 OFFSET $2`
	offset := (f.Page - 1) * f.PerPage
	rows, err := r.pool.Query(ctx, listSQL, f.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("admin media list: %w", err)
	}
	defer rows.Close()

	out := make([]Media, 0, f.PerPage)
	for rows.Next() {
		m, err := r.scan(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("admin media list scan: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin media list iterate: %w", err)
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM public.media`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin media count: %w", err)
	}
	return out, total, nil
}

// Get returns a single media row. Returns ErrNotFound when absent.
func (r *AdminRepository) Get(ctx context.Context, id uuid.UUID) (Media, error) {
	const q = `SELECT ` + adminSelectColumns + ` FROM public.media WHERE id = $1 LIMIT 1`
	row := r.pool.QueryRow(ctx, q, id)
	m, err := r.scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Media{}, ErrNotFound
	}
	if err != nil {
		return Media{}, fmt.Errorf("admin media get: %w", err)
	}
	return m, nil
}

// Create inserts a new media row and returns it.
func (r *AdminRepository) Create(ctx context.Context, in CreateInput) (Media, error) {
	id := uuid.New()
	const insert = `
        INSERT INTO public.media (
            id, filename, storage_path, mime_type, size,
            width, height, alt_text, uploaded_by
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := r.pool.Exec(ctx, insert,
		id, in.Filename, in.StoragePath, in.MimeType, in.Size,
		in.Width, in.Height, in.AltText, in.UploadedBy,
	); err != nil {
		return Media{}, fmt.Errorf("admin media insert: %w", err)
	}
	return r.Get(ctx, id)
}

// Delete removes a media row by id. Returns ErrNotFound when absent.
func (r *AdminRepository) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM public.media WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("admin media delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner abstracts pgx.Row and pgx.Rows for scan reuse.
type rowScanner interface {
	Scan(dest ...any) error
}

func (r *AdminRepository) scan(row rowScanner) (Media, error) {
	var m Media
	if err := row.Scan(
		&m.ID, &m.Filename, &m.StoragePath, &m.MimeType, &m.Size,
		&m.Width, &m.Height, &m.AltText, &m.UploadedBy, &m.CreatedAt,
	); err != nil {
		return Media{}, err
	}
	m.URL = r.urlPrefix + m.StoragePath
	return m, nil
}
