package model

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated user within an organization.
type User struct {
	BaseEntity
	OrganizationID uuid.UUID  `json:"organization_id" db:"organization_id"`
	Email          string     `json:"email" db:"email"`
	PasswordHash   string     `json:"-" db:"password_hash"`
	FirstName      string     `json:"first_name" db:"first_name"`
	LastName       string     `json:"last_name" db:"last_name"`
	Role           string     `json:"role" db:"role"`
	APIKeyHash     string     `json:"-" db:"api_key_hash"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	Active         bool       `json:"active" db:"active"`
}

// FullName returns the user's full display name.
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}
