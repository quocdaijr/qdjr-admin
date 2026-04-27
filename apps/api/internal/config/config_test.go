package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DefaultsAndRequired(t *testing.T) {
	t.Run("returns error when DATABASE_URL is missing", func(t *testing.T) {
		env := map[string]string{
			"SUPABASE_URL":        "http://localhost:54321",
			"SUPABASE_JWT_SECRET": "x",
		}
		_, err := Load(envGetter(env))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DATABASE_URL")
	})

	t.Run("loads defaults and parses CORS list", func(t *testing.T) {
		env := map[string]string{
			"DATABASE_URL":        "postgres://x",
			"SUPABASE_URL":        "http://localhost:54321",
			"SUPABASE_JWT_SECRET": "secret-secret-secret-secret-secret",
			"CORS_ORIGINS":        "http://a, http://b",
		}
		c, err := Load(envGetter(env))
		require.NoError(t, err)
		assert.Equal(t, "8080", c.Port)
		assert.Equal(t, "development", c.Env)
		assert.Equal(t, []string{"http://a", "http://b"}, c.CORSOrigins)
		assert.Equal(t, "secret-secret-secret-secret-secret", c.SupabaseJWTSecret)
	})
}

func envGetter(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}
