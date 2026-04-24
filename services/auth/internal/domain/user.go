package domain

import "time"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID            string
	Email         string
	PasswordHash  string
	Role          Role
	IsActive      bool
	EmailVerified bool
	MFAEnabled    bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type MFASecret struct {
	UserID    string
	Secret    string // stored encrypted
	IsEnabled bool
	CreatedAt time.Time
}

type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
