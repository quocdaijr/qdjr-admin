package contact

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists contact_messages rows.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a contact message and returns its server-generated id and
// timestamp. An empty CreateInput.IP is stored as SQL NULL so we don't write
// junk into the inet column.
func (r *Repository) Create(ctx context.Context, in CreateInput) (Created, error) {
	const q = `
        INSERT INTO public.contact_messages (name, email, subject, body, ip, user_agent)
        VALUES ($1, $2, $3, $4, $5::inet, $6)
        RETURNING id, created_at`

	// Pass *string for ip: nil → NULL, non-nil → cast to inet by Postgres.
	var ipArg *string
	if in.IP != "" {
		ip := in.IP
		ipArg = &ip
	}

	var out Created
	if err := r.pool.QueryRow(ctx, q,
		in.Name, in.Email, in.Subject, in.Body, ipArg, nullableText(in.UserAgent),
	).Scan(&out.ID, &out.CreatedAt); err != nil {
		return Created{}, fmt.Errorf("contact create: %w", err)
	}
	return out, nil
}

// nullableText returns nil for "" and a pointer otherwise. Used so empty
// user-agent strings are persisted as SQL NULL rather than empty text.
func nullableText(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
