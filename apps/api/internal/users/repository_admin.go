package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminRepository provides admin list/get/role-mutation for users. It joins
// auth.users with public.user_roles + public.roles.
type AdminRepository struct {
	pool *pgxpool.Pool
}

// NewAdminRepository constructs an AdminRepository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

// adminSelectColumns is the column list returned for List/Get. The LEFT JOIN
// means users without a role row have NULL role/assigned_at.
const adminSelectColumns = `
    u.id,
    u.email,
    r.name,
    u.last_sign_in_at,
    u.created_at,
    ur.assigned_at`

const adminFromClause = `
    FROM auth.users u
    LEFT JOIN public.user_roles ur ON ur.user_id = u.id
    LEFT JOIN public.roles r ON r.id = ur.role_id`

// List returns paginated user rows ordered by auth.users.created_at DESC.
func (r *AdminRepository) List(ctx context.Context, f ListFilter) ([]User, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}

	listSQL := `
        SELECT ` + adminSelectColumns + adminFromClause + `
        ORDER BY u.created_at DESC, u.id
        LIMIT $1 OFFSET $2`
	offset := (f.Page - 1) * f.PerPage
	rows, err := r.pool.Query(ctx, listSQL, f.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("admin users list: %w", err)
	}
	defer rows.Close()

	out := make([]User, 0, f.PerPage)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("admin users list scan: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin users list iterate: %w", err)
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM auth.users`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin users count: %w", err)
	}
	return out, total, nil
}

// Get returns a single user by id.
func (r *AdminRepository) Get(ctx context.Context, id uuid.UUID) (User, error) {
	q := `
        SELECT ` + adminSelectColumns + adminFromClause + `
        WHERE u.id = $1
        LIMIT 1`
	row := r.pool.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("admin users get: %w", err)
	}
	return u, nil
}

// SetRole upserts the user_roles row for userID to the given role name.
// Returns ErrNotFound if the role name doesn't exist in public.roles.
func (r *AdminRepository) SetRole(ctx context.Context, userID uuid.UUID, role string) error {
	const upsert = `
        INSERT INTO public.user_roles (user_id, role_id)
        SELECT $1, id FROM public.roles WHERE name = $2
        ON CONFLICT (user_id) DO UPDATE
            SET role_id = excluded.role_id, assigned_at = now()`
	cmd, err := r.pool.Exec(ctx, upsert, userID, role)
	if err != nil {
		return fmt.Errorf("admin users set role: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("admin users set role: role %q not found", role)
	}
	return nil
}

// DeleteRole removes the user_roles row for userID. No-op if absent.
func (r *AdminRepository) DeleteRole(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM public.user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("admin users delete role: %w", err)
	}
	return nil
}

// IsLastSuperAdmin reports whether userID is currently the only super_admin.
// Used to guard self-demote / self-delete operations.
func (r *AdminRepository) IsLastSuperAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	const q = `
        SELECT
            count(*) FILTER (WHERE r.name = 'super_admin') AS total,
            bool_or(ur.user_id = $1 AND r.name = 'super_admin') AS target_is_super
        FROM public.user_roles ur
        JOIN public.roles r ON r.id = ur.role_id`
	var total int
	var targetIsSuper *bool
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&total, &targetIsSuper); err != nil {
		return false, fmt.Errorf("admin users last super_admin check: %w", err)
	}
	if targetIsSuper == nil || !*targetIsSuper {
		return false, nil
	}
	return total <= 1, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var u User
	if err := row.Scan(
		&u.ID, &u.Email, &u.Role, &u.LastSignInAt, &u.CreatedAt, &u.AssignedAt,
	); err != nil {
		return User{}, err
	}
	return u, nil
}
