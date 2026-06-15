package model

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUnverified Role = "unverified"
	RolePassenger  Role = "passenger"
	RoleDriver     Role = "driver"
	RoleAdmin      Role = "admin"
)

type Company struct {
	ID          uuid.UUID `db:"id"           json:"id"`
	Name        string    `db:"name"         json:"name"`
	EmailDomain string    `db:"email_domain" json:"email_domain"`
	IsActive    bool      `db:"is_active"    json:"is_active"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`
}

type User struct {
	ID              uuid.UUID  `db:"id"               json:"id"`
	CompanyID       uuid.UUID  `db:"company_id"       json:"company_id"`
	Email           string     `db:"email"            json:"email"`
	PasswordHash    string     `db:"password_hash"    json:"-"`
	FullName        string     `db:"full_name"        json:"full_name"`
	Phone           *string    `db:"phone"            json:"phone,omitempty"`
	City            *string    `db:"city"             json:"city,omitempty"`
	Role            Role       `db:"role"             json:"role"`
	EmailVerified   bool       `db:"email_verified"   json:"email_verified"`
	VerifyToken     *string    `db:"verify_token"     json:"-"`
	VerifyTokenAt   *time.Time `db:"verify_token_at"  json:"-"`
	RefreshToken    *string    `db:"refresh_token"    json:"-"`
	RefreshTokenAt  *time.Time `db:"refresh_token_at" json:"-"`
	IsActive        bool       `db:"is_active"        json:"is_active"`
	CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"       json:"updated_at"`
}
