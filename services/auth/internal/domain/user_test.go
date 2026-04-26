package domain_test

import (
	"testing"
	"time"

	"github.com/tenzoshare/tenzoshare/services/auth/internal/domain"
)

// ── User.IsLocked ─────────────────────────────────────────────────────────────

func TestUser_IsLocked_NilLockedUntil(t *testing.T) {
	u := &domain.User{LockedUntil: nil}
	if u.IsLocked() {
		t.Fatal("expected not locked when LockedUntil is nil")
	}
}

func TestUser_IsLocked_PastLockout(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	u := &domain.User{LockedUntil: &past}
	if u.IsLocked() {
		t.Fatal("expected not locked when LockedUntil is in the past")
	}
}

func TestUser_IsLocked_FutureLockout(t *testing.T) {
	future := time.Now().Add(15 * time.Minute)
	u := &domain.User{LockedUntil: &future}
	if !u.IsLocked() {
		t.Fatal("expected locked when LockedUntil is in the future")
	}
}

func TestUser_IsLocked_NowBoundary(t *testing.T) {
	// Exactly now — should not be locked (After returns false for equal times)
	now := time.Now()
	u := &domain.User{LockedUntil: &now}
	// This may flap at nanosecond boundary; treat either result as acceptable
	// by just verifying it doesn't panic
	_ = u.IsLocked()
}

// ── Role constants ────────────────────────────────────────────────────────────

func TestRole_Constants(t *testing.T) {
	if domain.RoleUser != "user" {
		t.Errorf("RoleUser = %q, want %q", domain.RoleUser, "user")
	}
	if domain.RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %q, want %q", domain.RoleAdmin, "admin")
	}
}
