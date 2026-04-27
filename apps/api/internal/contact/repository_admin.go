package contact

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminRepository provides admin list/update for contact_messages.
type AdminRepository struct {
	pool *pgxpool.Pool
}

// NewAdminRepository constructs an AdminRepository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

// adminSelectColumns lists every column exposed in admin responses. host(ip)
// renders inet as a plain text address (or NULL).
const adminSelectColumns = `
    id, name, email, subject, body,
    CASE WHEN ip IS NULL THEN NULL ELSE host(ip)::text END AS ip,
    user_agent, status::text, created_at`

// List returns paginated contact messages, optionally filtered by status. Sort
// is created_at DESC.
func (r *AdminRepository) List(ctx context.Context, f AdminListFilter) ([]Message, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}
	var status *string
	if f.Status != "" {
		s := f.Status
		status = &s
	}

	const listSQL = `
        SELECT ` + adminSelectColumns + `
        FROM public.contact_messages
        WHERE ($1::public.contact_status IS NULL OR status = $1::public.contact_status)
        ORDER BY created_at DESC, id
        LIMIT $2 OFFSET $3`

	offset := (f.Page - 1) * f.PerPage
	rows, err := r.pool.Query(ctx, listSQL, status, f.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("admin contact list: %w", err)
	}
	defer rows.Close()

	out := make([]Message, 0, f.PerPage)
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Email, &m.Subject, &m.Body,
			&m.IP, &m.UserAgent, &m.Status, &m.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("admin contact list scan: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin contact list iterate: %w", err)
	}

	const countSQL = `
        SELECT count(*) FROM public.contact_messages
        WHERE ($1::public.contact_status IS NULL OR status = $1::public.contact_status)`
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin contact count: %w", err)
	}
	return out, total, nil
}

// UpdateStatus sets the status enum on a single message. Returns ErrNotFound
// when no row matches.
func (r *AdminRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (Message, error) {
	const stmt = `
        UPDATE public.contact_messages
        SET status = $1::public.contact_status
        WHERE id = $2`
	cmd, err := r.pool.Exec(ctx, stmt, status, id)
	if err != nil {
		return Message{}, fmt.Errorf("admin contact update status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return Message{}, ErrNotFound
	}
	return r.getByID(ctx, id)
}

func (r *AdminRepository) getByID(ctx context.Context, id uuid.UUID) (Message, error) {
	const q = `SELECT ` + adminSelectColumns + ` FROM public.contact_messages WHERE id = $1 LIMIT 1`
	var m Message
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&m.ID, &m.Name, &m.Email, &m.Subject, &m.Body,
		&m.IP, &m.UserAgent, &m.Status, &m.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, fmt.Errorf("admin contact get: %w", err)
	}
	return m, nil
}
