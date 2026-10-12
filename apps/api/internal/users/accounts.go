package users

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
)

// ErrEmailTaken is returned by Create when the email already has an account.
var ErrEmailTaken = errors.New("users: email already exists")

// ErrInvalidEmail is returned by Create for a malformed address.
var ErrInvalidEmail = errors.New("users: email is not a valid address")

// Credential is what login needs: identity plus the bcrypt hash.
type Credential struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
}

// PGAccounts owns rows in public.users: creation with a bcrypt hash, lookup
// for login, password changes and deletion (user_roles cascades).
type PGAccounts struct {
	pool *pgxpool.Pool
}

// NewPGAccounts constructs a PGAccounts.
func NewPGAccounts(pool *pgxpool.Pool) *PGAccounts {
	return &PGAccounts{pool: pool}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Create inserts a new account and returns its id.
func (a *PGAccounts) Create(ctx context.Context, email, password string) (uuid.UUID, error) {
	email = normalizeEmail(email)
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return uuid.Nil, ErrInvalidEmail
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = a.pool.QueryRow(ctx,
		`insert into public.users (email, password_hash) values ($1, $2) returning id`,
		email, hash,
	).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return uuid.Nil, ErrEmailTaken
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

// Delete removes the account. A missing id is not an error.
func (a *PGAccounts) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := a.pool.Exec(ctx, `delete from public.users where id = $1`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// FindByEmail looks an account up case-insensitively.
func (a *PGAccounts) FindByEmail(ctx context.Context, email string) (Credential, error) {
	return a.find(ctx, `where email = $1`, normalizeEmail(email))
}

// FindByID looks an account up by id.
func (a *PGAccounts) FindByID(ctx context.Context, id uuid.UUID) (Credential, error) {
	return a.find(ctx, `where id = $1`, id)
}

func (a *PGAccounts) find(ctx context.Context, where string, arg any) (Credential, error) {
	var c Credential
	err := a.pool.QueryRow(ctx,
		`select id, email, password_hash from public.users `+where, arg,
	).Scan(&c.ID, &c.Email, &c.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("find user: %w", err)
	}
	return c, nil
}

// SetPassword stores a new bcrypt hash.
func (a *PGAccounts) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := a.pool.Exec(ctx,
		`update public.users set password_hash = $2, updated_at = now() where id = $1`, id, hash)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchSignIn records a successful login.
func (a *PGAccounts) TouchSignIn(ctx context.Context, id uuid.UUID) error {
	if _, err := a.pool.Exec(ctx, `update public.users set last_sign_in_at = now() where id = $1`, id); err != nil {
		return fmt.Errorf("touch sign-in: %w", err)
	}
	return nil
}
