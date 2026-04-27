package auth

import (
	"errors"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the subset of JWT claims this service consumes.
type Claims struct {
	UserID uuid.UUID
	Email  string
}

// Verifier turns a raw bearer token into Claims, or returns an error.
type Verifier interface {
	Verify(token string) (Claims, error)
}

// NewHS256Verifier verifies tokens issued by Supabase's legacy shared-secret model.
func NewHS256Verifier(secret []byte) Verifier {
	return &hsVerifier{secret: secret}
}

// NewJWKSVerifier verifies tokens issued under Supabase's asymmetric key model.
// Pass the project's JWKS URL (e.g., https://<project-ref>.supabase.co/auth/v1/.well-known/jwks.json).
func NewJWKSVerifier(jwksURL string) (Verifier, error) {
	k, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("init jwks: %w", err)
	}
	return &jwksVerifier{keyfunc: k.Keyfunc}, nil
}

type hsVerifier struct{ secret []byte }

func (v *hsVerifier) Verify(token string) (Claims, error) {
	return verifyWith(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.secret, nil
	})
}

type jwksVerifier struct{ keyfunc jwt.Keyfunc }

func (v *jwksVerifier) Verify(token string) (Claims, error) {
	return verifyWith(token, v.keyfunc)
}

func verifyWith(token string, kf jwt.Keyfunc) (Claims, error) {
	parsed, err := jwt.Parse(token, kf, jwt.WithValidMethods([]string{"HS256", "RS256", "ES256"}))
	if err != nil {
		return Claims{}, err
	}
	if !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, errors.New("unexpected claims type")
	}
	sub, _ := mc["sub"].(string)
	uid, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub: %w", err)
	}
	email, _ := mc["email"].(string)
	return Claims{UserID: uid, Email: email}, nil
}
