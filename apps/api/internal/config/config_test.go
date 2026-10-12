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

func TestLoad_MediaStorage(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret}
	with := func(extra map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	t.Run("development defaults to a local folder served by the API", func(t *testing.T) {
		c, err := Load(envGetter(with(map[string]string{"PORT": "9000"})))
		require.NoError(t, err)
		assert.False(t, c.R2Configured())
		assert.Equal(t, "tmp/media", c.MediaDir)
		assert.Equal(t, "http://localhost:9000/media-files", c.MediaPublicBaseURL)
	})

	t.Run("production requires R2 and a public base URL", func(t *testing.T) {
		_, err := Load(envGetter(with(map[string]string{"ENV": "production"})))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "R2_")
		assert.Contains(t, err.Error(), "MEDIA_PUBLIC_BASE_URL")
	})

	t.Run("R2 configured; trailing slash trimmed", func(t *testing.T) {
		c, err := Load(envGetter(with(map[string]string{
			"ENV":                   "production",
			"R2_ACCOUNT_ID":         "acct",
			"R2_ACCESS_KEY_ID":      "id",
			"R2_SECRET_ACCESS_KEY":  "secret",
			"R2_BUCKET":             "qdjr-media",
			"MEDIA_PUBLIC_BASE_URL": "https://pub-abc.r2.dev/",
		})))
		require.NoError(t, err)
		assert.True(t, c.R2Configured())
		assert.Equal(t, "https://pub-abc.r2.dev", c.MediaPublicBaseURL)
	})

	t.Run("partial R2 config is an error, not a silent local fallback", func(t *testing.T) {
		_, err := Load(envGetter(with(map[string]string{"R2_ACCOUNT_ID": "acct"})))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "R2_")
	})
}
