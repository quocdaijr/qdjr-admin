// Package auth issues and verifies the API's own JWTs (HS256) and hashes
// admin passwords. Access and refresh tokens share one secret and are told
// apart by the "typ" claim.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Token types carried in the "typ" claim.
const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

// ErrWrongTokenType is returned when a refresh token is presented where an
// access token is required, or the other way round.
var ErrWrongTokenType = errors.New("auth: wrong token type")

// Claims is the subset of JWT claims this service consumes.
type Claims struct {
	UserID uuid.UUID
	Email  string
}

// Verifier turns a raw bearer token into Claims, or returns an error. It is
// what the HTTP middleware depends on.
type Verifier interface {
	Verify(token string) (Claims, error)
}

// TokenPair is the login / refresh response body.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // access token lifetime, seconds
}

// Issuer signs and verifies the API's tokens.
type Issuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time // injectable clock for tests
}

// NewIssuer returns an Issuer. secret must be at least 32 bytes (enforced by
// config.Load).
func NewIssuer(secret []byte, accessTTL, refreshTTL time.Duration) *Issuer {
	return &Issuer{secret: secret, accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now}
}

// Issue mints a fresh access + refresh pair for the user.
func (i *Issuer) Issue(userID uuid.UUID, email string) (TokenPair, error) {
	access, err := i.sign(userID, email, TypeAccess, i.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := i.sign(userID, email, TypeRefresh, i.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(i.accessTTL / time.Second),
	}, nil
}

// Verify accepts access tokens only (implements Verifier).
func (i *Issuer) Verify(token string) (Claims, error) {
	return i.verify(token, TypeAccess)
}

// VerifyRefresh accepts refresh tokens only.
func (i *Issuer) VerifyRefresh(token string) (Claims, error) {
	return i.verify(token, TypeRefresh)
}

func (i *Issuer) sign(userID uuid.UUID, email, typ string, ttl time.Duration) (string, error) {
	now := i.now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"typ":   typ,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"jti":   uuid.NewString(),
	})
	signed, err := tok.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("sign %s token: %w", typ, err)
	}
	return signed, nil
}

func (i *Issuer) verify(token, want string) (Claims, error) {
	mc, err := parseHS256(token, i.secret)
	if err != nil {
		return Claims{}, err
	}
	if typ, _ := mc["typ"].(string); typ != want {
		return Claims{}, ErrWrongTokenType
	}
	return claimsFrom(mc)
}

// NewHS256Verifier verifies access tokens signed with secret. Tokens without
// a "typ" claim are accepted (test fixtures mint them); refresh tokens are not.
func NewHS256Verifier(secret []byte) Verifier {
	return hsVerifier{secret: secret}
}

type hsVerifier struct{ secret []byte }

func (v hsVerifier) Verify(token string) (Claims, error) {
	mc, err := parseHS256(token, v.secret)
	if err != nil {
		return Claims{}, err
	}
	if typ, _ := mc["typ"].(string); typ == TypeRefresh {
		return Claims{}, ErrWrongTokenType
	}
	return claimsFrom(mc)
}

func parseHS256(token string, secret []byte) (jwt.MapClaims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return mc, nil
}

func claimsFrom(mc jwt.MapClaims) (Claims, error) {
	sub, _ := mc["sub"].(string)
	uid, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub: %w", err)
	}
	email, _ := mc["email"].(string)
	return Claims{UserID: uid, Email: email}, nil
}
