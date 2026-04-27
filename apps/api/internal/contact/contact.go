// Package contact implements the public-facing contact form endpoint.
//
// Submissions are rate-limited per IP and persisted to public.contact_messages
// for later admin review.
package contact

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Validation limits. Kept tight to discourage abuse: the form is unauth.
const (
	maxNameLen    = 200
	maxEmailLen   = 200
	maxSubjectLen = 200
	maxBodyLen    = 5000
)

// Input is the JSON body accepted by POST /v1/contact.
type Input struct {
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Subject *string `json:"subject"`
	Body    string  `json:"body"`
}

// CreateInput is what the handler hands to the repository after trimming and
// validation. IP and UserAgent come from the HTTP context, not the JSON body.
type CreateInput struct {
	Name      string
	Email     string
	Subject   *string
	Body      string
	IP        string // empty means do not record
	UserAgent string
}

// Created is the repository's return value after a successful insert.
type Created struct {
	ID        uuid.UUID
	CreatedAt time.Time
}

// Validate trims whitespace in-place and enforces field constraints.
//
// On any constraint violation it returns a user-readable error suitable for
// surfacing in a 400 response.
func (in *Input) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Body = strings.TrimSpace(in.Body)
	if in.Subject != nil {
		s := strings.TrimSpace(*in.Subject)
		if s == "" {
			in.Subject = nil
		} else {
			in.Subject = &s
		}
	}

	switch {
	case in.Name == "":
		return errors.New("name is required")
	case len(in.Name) > maxNameLen:
		return errors.New("name is too long")
	case in.Email == "":
		return errors.New("email is required")
	case len(in.Email) > maxEmailLen:
		return errors.New("email is too long")
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return errors.New("email is invalid")
	}
	if in.Subject != nil && len(*in.Subject) > maxSubjectLen {
		return errors.New("subject is too long")
	}
	switch {
	case in.Body == "":
		return errors.New("body is required")
	case len(in.Body) > maxBodyLen:
		return errors.New("body is too long")
	}
	return nil
}

// Writer is the contract handlers depend on so they can be tested without DB.
type Writer interface {
	Create(ctx context.Context, in CreateInput) (Created, error)
}
