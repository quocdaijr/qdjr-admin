package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goodSecret = "0123456789abcdef0123456789abcdef"

func TestLoad_DefaultsAndRequired(t *testing.T) {
	t.Run("lists every missing required var at once", func(t *testing.T) {
		_, err := Load(envGetter(map[string]string{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DATABASE_URL")
		assert.Contains(t, err.Error(), "JWT_SECRET")
	})

	t.Run("loads defaults and parses CORS list", func(t *testing.T) {
		c, err := Load(envGetter(map[string]string{
			"DATABASE_URL": "postgres://x",
			"JWT_SECRET":   goodSecret,
			"CORS_ORIGINS": "http://a, http://b",
		}))
		require.NoError(t, err)
		assert.Equal(t, "8080", c.Port)
		assert.Equal(t, "development", c.Env)
		assert.Equal(t, []string{"http://a", "http://b"}, c.CORSOrigins)
		assert.Equal(t, goodSecret, c.JWTSecret)
		assert.Equal(t, 15*time.Minute, c.AccessTTL)
		assert.Equal(t, 7*24*time.Hour, c.RefreshTTL)
		assert.Equal(t, "postgres://x", c.MigrationsDatabaseURL, "falls back to DATABASE_URL")
	})

	t.Run("direct migrations URL and TTLs override", func(t *testing.T) {
		c, err := Load(envGetter(map[string]string{
			"DATABASE_URL":            "postgres://pooled",
			"MIGRATIONS_DATABASE_URL": "postgres://direct",
			"JWT_SECRET":              goodSecret,
			"JWT_ACCESS_TTL":          "5m",
			"JWT_REFRESH_TTL":         "24h",
		}))
		require.NoError(t, err)
		assert.Equal(t, "postgres://direct", c.MigrationsDatabaseURL)
		assert.Equal(t, 5*time.Minute, c.AccessTTL)
		assert.Equal(t, 24*time.Hour, c.RefreshTTL)
	})
}

func TestLoad_RejectsWeakOrMalformedSecrets(t *testing.T) {
	_, err := Load(envGetter(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": "too-short"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32")

	_, err = Load(envGetter(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat(" ", 40)}))
	require.Error(t, err, "whitespace-only secret")

	_, err = Load(envGetter(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret, "JWT_ACCESS_TTL": "soon"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_ACCESS_TTL")
}

func envGetter(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}
