package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Password length bounds. bcrypt reads at most 72 bytes, so longer input
// would be silently truncated; reject it instead.
const (
	MinPasswordLen = 10
	MaxPasswordLen = 72
	bcryptCost     = 12
)

// ErrWeakPassword is returned by ValidatePassword.
var ErrWeakPassword = fmt.Errorf("password must be %d-%d bytes", MinPasswordLen, MaxPasswordLen)

// ValidatePassword checks length only; composition rules add friction without
// adding much for an admin tool with a handful of users.
func ValidatePassword(pw string) error {
	if n := len(pw); n < MinPasswordLen || n > MaxPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

// HashPassword returns a bcrypt hash of pw.
func HashPassword(pw string) (string, error) {
	if err := ValidatePassword(pw); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword reports whether pw matches hash. Any malformed hash is a
// mismatch, never an error the caller could leak.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// DummyHash is compared against when the email is unknown, so a login for a
// missing account costs the same bcrypt time as a wrong password.
var DummyHash = func() string {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
	if err != nil {
		panic(errors.New("auth: cannot build dummy hash"))
	}
	return string(h)
}()
