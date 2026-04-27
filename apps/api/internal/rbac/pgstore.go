package rbac

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct{ pool *pgxpool.Pool }

// NewPGStore returns a RoleStore backed by Postgres (joins user_roles → roles).
func NewPGStore(p *pgxpool.Pool) RoleStore { return &pgStore{pool: p} }

func (s *pgStore) RoleForUser(ctx context.Context, u uuid.UUID) (string, error) {
	const q = `select r.name
                 from public.user_roles ur
                 join public.roles r on r.id = ur.role_id
                where ur.user_id = $1`
	var name string
	err := s.pool.QueryRow(ctx, q, u).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoRole
	}
	return name, err
}
