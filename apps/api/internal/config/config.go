package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds runtime configuration. All fields are populated from env vars.
type Config struct {
	Port                   string
	Env                    string
	DatabaseURL            string
	SupabaseURL            string
	SupabaseJWTSecret      string
	SupabaseJWKSURL        string
	SupabaseServiceRoleKey string
	CORSOrigins            []string
}

// Load reads configuration from the supplied env getter (use os.Getenv in main).
// Returns an error if a required value is missing.
func Load(get func(string) string) (Config, error) {
	c := Config{
		Port:                   getOr(get, "PORT", "8080"),
		Env:                    getOr(get, "ENV", "development"),
		DatabaseURL:            get("DATABASE_URL"),
		SupabaseURL:            get("SUPABASE_URL"),
		SupabaseJWTSecret:      get("SUPABASE_JWT_SECRET"),
		SupabaseJWKSURL:        get("SUPABASE_JWKS_URL"),
		SupabaseServiceRoleKey: get("SUPABASE_SERVICE_ROLE_KEY"),
		CORSOrigins:            parseCSV(get("CORS_ORIGINS")),
	}
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.SupabaseURL == "" {
		missing = append(missing, "SUPABASE_URL")
	}
	if c.SupabaseJWTSecret == "" && c.SupabaseJWKSURL == "" {
		missing = append(missing, "SUPABASE_JWT_SECRET or SUPABASE_JWKS_URL")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

// LoadFromOS is a convenience wrapper for main().
func LoadFromOS() (Config, error) { return Load(os.Getenv) }

func getOr(get func(string) string, key, fallback string) string {
	if v := get(key); v != "" {
		return v
	}
	return fallback
}

func parseCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
