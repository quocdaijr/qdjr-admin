package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyHS256_ValidToken(t *testing.T) {
	secret := "test-secret-test-secret-test-secret"
	userID := uuid.New()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID.String(),
		"email": "alice@example.com",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)

	v := NewHS256Verifier([]byte(secret))
	claims, err := v.Verify(signed)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "alice@example.com", claims.Email)
}

func TestVerifyHS256_RejectsExpired(t *testing.T) {
	secret := "test-secret-test-secret-test-secret"
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uuid.New().String(),
		"exp": time.Now().Add(-1 * time.Minute).Unix(),
	})
	signed, _ := tok.SignedString([]byte(secret))

	v := NewHS256Verifier([]byte(secret))
	_, err := v.Verify(signed)
	assert.Error(t, err)
}

func TestVerifyHS256_RejectsBadSignature(t *testing.T) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uuid.New().String(),
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	signed, _ := tok.SignedString([]byte("wrong-secret-wrong-secret-wrong"))

	v := NewHS256Verifier([]byte("right-secret-right-secret-right"))
	_, err := v.Verify(signed)
	assert.Error(t, err)
}
