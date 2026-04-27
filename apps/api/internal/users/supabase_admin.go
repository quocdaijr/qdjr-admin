package users

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SupabaseAdminClient calls the Supabase Auth Admin API to manage auth.users.
// It uses the service-role key for authorization.
type SupabaseAdminClient struct {
	baseURL        string // e.g. "http://127.0.0.1:54321"
	serviceRoleKey string
	hc             *http.Client
}

// NewSupabaseAdminClient constructs a SupabaseAdminClient.
func NewSupabaseAdminClient(baseURL, serviceRoleKey string) *SupabaseAdminClient {
	return &SupabaseAdminClient{
		baseURL:        strings.TrimRight(baseURL, "/"),
		serviceRoleKey: serviceRoleKey,
		hc:             &http.Client{Timeout: 15 * time.Second},
	}
}

// EnsureUser returns the existing auth.users id for email, or creates a new
// one. password is required when creating a new user; it is ignored when an
// existing user is found.
func (c *SupabaseAdminClient) EnsureUser(ctx context.Context, email, password string) (uuid.UUID, error) {
	id, found, err := c.findByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		return id, nil
	}
	if password == "" {
		return uuid.Nil, fmt.Errorf("users supabase: password required to create new user")
	}
	return c.createUser(ctx, email, password)
}

// DeleteUser removes the auth.users row by id. A 404 from Supabase is treated
// as success (idempotent delete).
func (c *SupabaseAdminClient) DeleteUser(ctx context.Context, id uuid.UUID) error {
	endpoint := c.baseURL + "/auth/v1/admin/users/" + id.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("users supabase: build delete request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("users supabase: delete request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("users supabase: delete: status=%d body=%s", resp.StatusCode, string(body))
}

// findByEmail queries the admin users endpoint filtered by email. Returns the
// uuid and true if found.
func (c *SupabaseAdminClient) findByEmail(ctx context.Context, email string) (uuid.UUID, bool, error) {
	endpoint := c.baseURL + "/auth/v1/admin/users?email=" + url.QueryEscape(email)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("users supabase: build find request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.hc.Do(req)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("users supabase: find request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return uuid.Nil, false, fmt.Errorf("users supabase: find: status=%d body=%s",
			resp.StatusCode, string(body))
	}
	var listed struct {
		Users []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		return uuid.Nil, false, fmt.Errorf("users supabase: decode find response: %w", err)
	}
	for _, u := range listed.Users {
		if strings.EqualFold(u.Email, email) {
			id, err := uuid.Parse(u.ID)
			if err != nil {
				return uuid.Nil, false, fmt.Errorf("users supabase: invalid uuid %q: %w", u.ID, err)
			}
			return id, true, nil
		}
	}
	return uuid.Nil, false, nil
}

// createUser POSTs to the admin users endpoint to create an email-confirmed user.
func (c *SupabaseAdminClient) createUser(ctx context.Context, email, password string) (uuid.UUID, error) {
	endpoint := c.baseURL + "/auth/v1/admin/users"
	payload, err := json.Marshal(map[string]any{
		"email":         email,
		"password":      password,
		"email_confirm": true,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("users supabase: marshal create body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return uuid.Nil, fmt.Errorf("users supabase: build create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("users supabase: create request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return uuid.Nil, fmt.Errorf("users supabase: create: status=%d body=%s",
			resp.StatusCode, string(body))
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return uuid.Nil, fmt.Errorf("users supabase: decode create response: %w", err)
	}
	id, err := uuid.Parse(created.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("users supabase: invalid uuid %q: %w", created.ID, err)
	}
	return id, nil
}

func (c *SupabaseAdminClient) setAuth(req *http.Request) {
	req.Header.Set("apikey", c.serviceRoleKey)
	req.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
}
