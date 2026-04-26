package domain_test

import (
	"testing"
	"time"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
)

// ── Transfer.Status ───────────────────────────────────────────────────────────

func TestTransfer_Status_Active(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp}
	if got := tr.Status(); got != "active" {
		t.Errorf("Status() = %q, want %q", got, "active")
	}
}

func TestTransfer_Status_Revoked(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: true, ExpiresAt: &exp}
	if got := tr.Status(); got != "revoked" {
		t.Errorf("Status() = %q, want %q", got, "revoked")
	}
}

func TestTransfer_Status_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &past}
	if got := tr.Status(); got != "expired" {
		t.Errorf("Status() = %q, want %q", got, "expired")
	}
}

func TestTransfer_Status_RevokedAndExpired(t *testing.T) {
	// Revoked takes precedence over expired
	past := time.Now().Add(-1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: true, ExpiresAt: &past}
	if got := tr.Status(); got != "revoked" {
		t.Errorf("Status() = %q, want %q (revoked should take precedence)", got, "revoked")
	}
}

func TestTransfer_Status_NilExpiresAt(t *testing.T) {
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: nil}
	if got := tr.Status(); got != "active" {
		t.Errorf("Status() = %q, want %q (nil ExpiresAt should be active)", got, "active")
	}
}
