package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	require.NoError(t, err)
	assert.NotContains(t, hash, "correct horse")
	assert.True(t, CheckPassword(hash, "correct horse battery"))
	assert.False(t, CheckPassword(hash, "correct horse batterY"))
	assert.False(t, CheckPassword("not-a-bcrypt-hash", "x"))
}

func TestValidatePassword(t *testing.T) {
	assert.Error(t, ValidatePassword(""))
	assert.Error(t, ValidatePassword("short9chr"))
	assert.NoError(t, ValidatePassword("tenchars10"))
	// bcrypt only reads 72 bytes; longer input would silently be truncated.
	assert.NoError(t, ValidatePassword(strings.Repeat("a", 72)))
	assert.Error(t, ValidatePassword(strings.Repeat("a", 73)))
}
