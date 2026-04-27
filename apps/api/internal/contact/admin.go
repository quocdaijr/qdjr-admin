package contact

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a contact message id is not found.
var ErrNotFound = errors.New("contact: not found")

// ValidStatuses lists the allowed values of public.contact_status. Mirrors the
// enum declared in supabase/migrations/0001_init_enums.sql.
var ValidStatuses = []string{"new", "read", "replied", "spam"}

// IsValidStatus reports whether s is one of the enum values.
func IsValidStatus(s string) bool {
	for _, v := range ValidStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// Message is the admin response shape for a contact_messages row.
type Message struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Subject   *string   `json:"subject"`
	Body      string    `json:"body"`
	IP        *string   `json:"ip"`
	UserAgent *string   `json:"user_agent"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminListFilter narrows admin list queries.
type AdminListFilter struct {
	Page    int
	PerPage int
	Status  string // optional single status; "" means no filter
}

// AdminWriter is the contract the admin handler depends on.
type AdminWriter interface {
	List(ctx context.Context, f AdminListFilter) ([]Message, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) (Message, error)
}
