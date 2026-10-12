package users

import (
	"context"
	"errors"
	"fmt"
)

// SeedAdmin creates email as a super_admin when no account with that email
// exists. It never changes an existing account (password or role), so it is
// safe to run on every API start. Empty email or password is a no-op.
func SeedAdmin(ctx context.Context, acc *PGAccounts, repo AdminWriter, email, password string) (bool, error) {
	if email == "" || password == "" {
		return false, nil
	}
	_, err := acc.FindByEmail(ctx, email)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return false, fmt.Errorf("seed admin lookup: %w", err)
	}
	id, err := acc.Create(ctx, email, password)
	if errors.Is(err, ErrEmailTaken) { // another instance won the race
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("seed admin create: %w", err)
	}
	if err := repo.SetRole(ctx, id, "super_admin"); err != nil {
		return false, fmt.Errorf("seed admin role: %w", err)
	}
	return true, nil
}
