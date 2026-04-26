package domain_test

import (
	"testing"
	"time"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
)

// ── FileRequest.IsExpired ─────────────────────────────────────────────────────

func TestFileRequest_IsExpired_ActiveNotExpired(t *testing.T) {
	r := &domain.FileRequest{
		IsActive:  true,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if r.IsExpired() {
		t.Fatal("expected not expired: active with future expiry")
	}
}

func TestFileRequest_IsExpired_PassedExpiry(t *testing.T) {
	r := &domain.FileRequest{
		IsActive:  true,
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	if !r.IsExpired() {
		t.Fatal("expected expired: past expiry time")
	}
}

func TestFileRequest_IsExpired_Deactivated(t *testing.T) {
	r := &domain.FileRequest{
		IsActive:  false,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if !r.IsExpired() {
		t.Fatal("expected expired: deactivated request")
	}
}

func TestFileRequest_IsExpired_DeactivatedAndPastExpiry(t *testing.T) {
	r := &domain.FileRequest{
		IsActive:  false,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if !r.IsExpired() {
		t.Fatal("expected expired: deactivated and past expiry")
	}
}
