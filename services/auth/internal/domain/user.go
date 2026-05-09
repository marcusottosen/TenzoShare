package domain

import "time"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID                  string
	Email               string
	PasswordHash        string
	Role                Role
	IsActive            bool
	EmailVerified       bool
	MFAEnabled          bool
	FailedLoginAttempts int
	LockedUntil         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	// Per-user format prefs; nil = use system default from platform_settings.
	DateFormat *string
	TimeFormat *string
	Timezone   *string
}

// IsLocked reports whether the account is currently locked out.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
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

type APIKey struct {
	ID        string
	UserID    string
	Name      string
	KeyPrefix string // first 12 chars of raw key — displayed in UI
	KeyHash   string // SHA-256(rawKey) — used for lookup/validation; raw key never stored
	LastUsed  *time.Time
	ExpiresAt *time.Time
	CreatedAt time.Time
}
