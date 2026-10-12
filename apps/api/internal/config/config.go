// Package config loads the API configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// MinJWTSecretLen is the shortest JWT_SECRET accepted (bytes).
const MinJWTSecretLen = 32

// Config holds runtime configuration. All fields are populated from env vars.
type Config struct {
	Port string
	Env  string
	// DatabaseURL is the runtime pool (Neon: the "-pooler" URL).
	DatabaseURL string
	// MigrationsDatabaseURL runs goose (Neon: the direct URL, because session
	// advisory locks are unreliable through PgBouncer). Defaults to DatabaseURL.
	MigrationsDatabaseURL string
	CORSOrigins           []string
	// RunMigrations applies embedded goose migrations before serving
	// (RUN_MIGRATIONS=true). On in production, off locally.
	RunMigrations bool

	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	// BootstrapAdminEmail/Password seed the first super_admin at start when
	// that email has no account yet. Leave empty once seeded.
	BootstrapAdminEmail    string
	BootstrapAdminPassword string

	// Supabase Storage (media) — removed when media moves to R2.
	SupabaseURL            string
	SupabaseServiceRoleKey string
}

// Load reads configuration from the supplied env getter (use os.Getenv in main).
// It reports every problem at once.
func Load(get func(string) string) (Config, error) {
	c := Config{
		Port:                   getOr(get, "PORT", "8080"),
		Env:                    getOr(get, "ENV", "development"),
		DatabaseURL:            strings.TrimSpace(get("DATABASE_URL")),
		MigrationsDatabaseURL:  strings.TrimSpace(get("MIGRATIONS_DATABASE_URL")),
		CORSOrigins:            parseCSV(get("CORS_ORIGINS")),
		RunMigrations:          get("RUN_MIGRATIONS") == "true",
		JWTSecret:              get("JWT_SECRET"),
		BootstrapAdminEmail:    strings.TrimSpace(get("BOOTSTRAP_ADMIN_EMAIL")),
		BootstrapAdminPassword: get("BOOTSTRAP_ADMIN_PASSWORD"),
		SupabaseURL:            get("SUPABASE_URL"),
		SupabaseServiceRoleKey: get("SUPABASE_SERVICE_ROLE_KEY"),
	}
	if c.MigrationsDatabaseURL == "" {
		c.MigrationsDatabaseURL = c.DatabaseURL
	}

	var problems []string
	if c.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	}
	switch secret := strings.TrimSpace(c.JWTSecret); {
	case secret == "":
		problems = append(problems, "JWT_SECRET is required")
	case len(secret) < MinJWTSecretLen:
		problems = append(problems, fmt.Sprintf("JWT_SECRET must be at least %d bytes", MinJWTSecretLen))
	}
	var err error
	if c.AccessTTL, err = durationOr(get, "JWT_ACCESS_TTL", 15*time.Minute); err != nil {
		problems = append(problems, err.Error())
	}
	if c.RefreshTTL, err = durationOr(get, "JWT_REFRESH_TTL", 7*24*time.Hour); err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid config: %s", strings.Join(problems, "; "))
	}
	return c, nil
}

// LoadFromOS is a convenience wrapper for main().
func LoadFromOS() (Config, error) { return Load(os.Getenv) }

func getOr(get func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(get(key)); v != "" {
		return v
	}
	return fallback
}

func durationOr(get func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(get(key))
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration like 15m or 168h", key)
	}
	return d, nil
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
