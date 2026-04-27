// apps/api/cmd/bootstrap/main.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/config"
)

// Bootstrap creates (or finds) a Supabase Auth user with the given email and
// records them as super_admin in public.user_roles. Idempotent.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.LoadFromOS()
	if err != nil {
		return err
	}
	email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
	if email == "" {
		return fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL is required")
	}
	if cfg.SupabaseServiceRoleKey == "" {
		return fmt.Errorf("SUPABASE_SERVICE_ROLE_KEY is required for bootstrap")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uid, err := ensureAuthUser(ctx, cfg, email, os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
	if err != nil {
		return fmt.Errorf("ensure auth user: %w", err)
	}
	log.Info("auth user ready", "user_id", uid, "email", email)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()

	const upsert = `
		insert into public.user_roles (user_id, role_id)
		select $1, id from public.roles where name = 'super_admin'
		on conflict (user_id) do update set role_id = excluded.role_id, assigned_at = now()`
	if _, err := pool.Exec(ctx, upsert, uid); err != nil {
		return fmt.Errorf("upsert role: %w", err)
	}
	log.Info("super_admin role assigned", "user_id", uid)
	return nil
}

// ensureAuthUser returns the existing auth.users id for `email`, or creates one.
func ensureAuthUser(ctx context.Context, cfg config.Config, email, password string) (uuid.UUID, error) {
	base := cfg.SupabaseURL + "/auth/v1/admin/users"

	// Try GET first (filter by email).
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"?email="+email, nil)
	req.Header.Set("apikey", cfg.SupabaseServiceRoleKey)
	req.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceRoleKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return uuid.Nil, err
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		var listed struct {
			Users []struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"users"`
		}
		if err := json.Unmarshal(body, &listed); err == nil {
			for _, u := range listed.Users {
				if u.Email == email {
					return uuid.Parse(u.ID)
				}
			}
		}
	}

	// Otherwise create.
	if password == "" {
		return uuid.Nil, fmt.Errorf("user not found and BOOTSTRAP_ADMIN_PASSWORD not set")
	}
	payload, _ := json.Marshal(map[string]any{
		"email":         email,
		"password":      password,
		"email_confirm": true,
	})
	creq, _ := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(payload))
	creq.Header.Set("apikey", cfg.SupabaseServiceRoleKey)
	creq.Header.Set("Authorization", "Bearer "+cfg.SupabaseServiceRoleKey)
	creq.Header.Set("Content-Type", "application/json")
	cres, err := http.DefaultClient.Do(creq)
	if err != nil {
		return uuid.Nil, err
	}
	cbody, _ := io.ReadAll(cres.Body)
	cres.Body.Close()
	if cres.StatusCode >= 300 {
		return uuid.Nil, fmt.Errorf("create user: %d %s", cres.StatusCode, string(cbody))
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cbody, &created); err != nil {
		return uuid.Nil, fmt.Errorf("decode create response: %w", err)
	}
	return uuid.Parse(created.ID)
}
