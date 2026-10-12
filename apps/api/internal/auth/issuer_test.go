package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const issuerSecret = "issuer-secret-issuer-secret-issuer-secret"

func TestIssuer_IssueAndVerifyAccess(t *testing.T) {
	iss := NewIssuer([]byte(issuerSecret), 15*time.Minute, 7*24*time.Hour)
	uid := uuid.New()

	pair, err := iss.Issue(uid, "admin@qdjr.dev")
	require.NoError(t, err)
	assert.Equal(t, "Bearer", pair.TokenType)
	assert.Equal(t, 900, pair.ExpiresIn)
	assert.NotEqual(t, pair.AccessToken, pair.RefreshToken)

	claims, err := iss.Verify(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uid, claims.UserID)
	assert.Equal(t, "admin@qdjr.dev", claims.Email)
}

func TestIssuer_AccessVerifierRejectsRefreshToken(t *testing.T) {
	iss := NewIssuer([]byte(issuerSecret), time.Minute, time.Hour)
	pair, err := iss.Issue(uuid.New(), "a@b.co")
	require.NoError(t, err)

	_, err = iss.Verify(pair.RefreshToken)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWrongTokenType)
}

func TestIssuer_VerifyRefreshRejectsAccessToken(t *testing.T) {
	iss := NewIssuer([]byte(issuerSecret), time.Minute, time.Hour)
	pair, err := iss.Issue(uuid.New(), "a@b.co")
	require.NoError(t, err)

	claims, err := iss.VerifyRefresh(pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, claims.UserID)

	_, err = iss.VerifyRefresh(pair.AccessToken)
	assert.ErrorIs(t, err, ErrWrongTokenType)
}

func TestIssuer_RejectsExpiredAndForeignSecret(t *testing.T) {
	past := NewIssuer([]byte(issuerSecret), time.Minute, time.Hour)
	past.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	pair, err := past.Issue(uuid.New(), "a@b.co")
	require.NoError(t, err)

	iss := NewIssuer([]byte(issuerSecret), time.Minute, time.Hour)
	_, err = iss.Verify(pair.AccessToken)
	assert.Error(t, err, "expired access token")
	_, err = iss.VerifyRefresh(pair.RefreshToken)
	assert.Error(t, err, "expired refresh token")

	other := NewIssuer([]byte("another-secret-another-secret-another"), time.Minute, time.Hour)
	fresh, err := other.Issue(uuid.New(), "a@b.co")
	require.NoError(t, err)
	_, err = iss.Verify(fresh.AccessToken)
	assert.Error(t, err, "token signed with a different secret")
}
