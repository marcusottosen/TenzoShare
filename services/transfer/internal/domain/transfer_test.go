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

func TestTransfer_Status_Exhausted(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp, MaxDownloads: 5, IsExhausted: true}
	if got := tr.Status(); got != "exhausted" {
		t.Errorf("Status() = %q, want %q (IsExhausted=true)", got, "exhausted")
	}
}

func TestTransfer_Status_ExhaustedOverLimit(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp, MaxDownloads: 3, IsExhausted: true}
	if got := tr.Status(); got != "exhausted" {
		t.Errorf("Status() = %q, want %q", got, "exhausted")
	}
}

func TestTransfer_Status_NotExhaustedWhenIsExhaustedFalse(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp, MaxDownloads: 5, IsExhausted: false}
	if got := tr.Status(); got != "active" {
		t.Errorf("Status() = %q, want %q (IsExhausted=false means still active)", got, "active")
	}
}

func TestTransfer_Status_UnlimitedNeverExhausted(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour)
	// MaxDownloads=0 means unlimited; even if IsExhausted is somehow true it is ignored
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp, MaxDownloads: 0, IsExhausted: true}
	if got := tr.Status(); got != "active" {
		t.Errorf("Status() = %q, want %q (MaxDownloads=0 means unlimited)", got, "active")
	}
}

func TestTransfer_Status_RevokedTakesPrecedenceOverExhausted(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: true, ExpiresAt: &exp, MaxDownloads: 5, IsExhausted: true}
	if got := tr.Status(); got != "revoked" {
		t.Errorf("Status() = %q, want %q (revoked takes precedence)", got, "revoked")
	}
}

func TestTransfer_Status_MultiFileExhausted(t *testing.T) {
	// IsExhausted=true because all 3 files hit their per-file limit of 1
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp, MaxDownloads: 1, IsExhausted: true}
	if got := tr.Status(); got != "exhausted" {
		t.Errorf("Status() = %q, want %q (all per-file limits reached)", got, "exhausted")
	}
}

func TestTransfer_Status_MultiFilePartialDownload(t *testing.T) {
	// limit=1, 3 files → effective limit 3; download_count=2 means still active
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp, MaxDownloads: 1, DownloadCount: 2, FileCount: 3}
	if got := tr.Status(); got != "active" {
		t.Errorf("Status() = %q, want %q (one file still downloadable)", got, "active")
	}
}

func TestTransfer_Status_FileCountZeroSkipsExhaustedCheck(t *testing.T) {
	// FileCount=0 means unknown; should not trigger exhausted
	exp := time.Now().Add(1 * time.Hour)
	tr := &domain.Transfer{IsRevoked: false, ExpiresAt: &exp, MaxDownloads: 1, DownloadCount: 999, FileCount: 0}
	if got := tr.Status(); got != "active" {
		t.Errorf("Status() = %q, want %q (FileCount=0 should not trigger exhausted)", got, "active")
	}
}
