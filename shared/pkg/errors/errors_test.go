package errors_test

import (
	"errors"
	"net/http"
	"testing"

	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── Constructor HTTP status codes ─────────────────────────────────────────────

func TestNotFound(t *testing.T) {
	err := apperrors.NotFound("thing not found")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusNotFound)
	}
	if ae.Code != apperrors.CodeNotFound {
		t.Fatalf("code: got %q, want %q", ae.Code, apperrors.CodeNotFound)
	}
}

func TestUnauthorized(t *testing.T) {
	err := apperrors.Unauthorized("bad token")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusUnauthorized)
	}
}

func TestForbidden(t *testing.T) {
	err := apperrors.Forbidden("not your resource")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusForbidden)
	}
}

func TestConflict(t *testing.T) {
	err := apperrors.Conflict("already exists")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusConflict {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusConflict)
	}
}

func TestBadRequest(t *testing.T) {
	err := apperrors.BadRequest("invalid input")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusBadRequest)
	}
}

func TestValidation(t *testing.T) {
	err := apperrors.Validation("email required")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusBadRequest)
	}
	if ae.Code != apperrors.CodeValidation {
		t.Fatalf("code: got %q, want %q", ae.Code, apperrors.CodeValidation)
	}
}

func TestRateLimit(t *testing.T) {
	err := apperrors.RateLimit("slow down")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusTooManyRequests {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusTooManyRequests)
	}
}

func TestInternal(t *testing.T) {
	cause := errors.New("db connection lost")
	err := apperrors.Internal("query failed", cause)
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatal("expected *AppError")
	}
	if ae.Status != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", ae.Status, http.StatusInternalServerError)
	}
	// Unwrap should expose the original cause
	if !errors.Is(err, cause) {
		t.Fatal("expected cause to be unwrappable")
	}
}

// ── Sentinel helpers ──────────────────────────────────────────────────────────

func TestIsNotFound(t *testing.T) {
	if !apperrors.IsNotFound(apperrors.NotFound("x")) {
		t.Fatal("IsNotFound should return true for NotFound error")
	}
	if apperrors.IsNotFound(apperrors.Unauthorized("x")) {
		t.Fatal("IsNotFound should return false for Unauthorized")
	}
	if apperrors.IsNotFound(errors.New("plain error")) {
		t.Fatal("IsNotFound should return false for non-AppError")
	}
}

func TestIsUnauthorized(t *testing.T) {
	if !apperrors.IsUnauthorized(apperrors.Unauthorized("x")) {
		t.Fatal("IsUnauthorized should return true")
	}
}

func TestIsForbidden(t *testing.T) {
	if !apperrors.IsForbidden(apperrors.Forbidden("x")) {
		t.Fatal("IsForbidden should return true")
	}
}

func TestIsConflict(t *testing.T) {
	if !apperrors.IsConflict(apperrors.Conflict("x")) {
		t.Fatal("IsConflict should return true")
	}
}

// ── Error message includes cause ──────────────────────────────────────────────

func TestInternalErrorMessage(t *testing.T) {
	cause := errors.New("underlying")
	err := apperrors.Internal("wrap", cause)
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error string")
	}
	if !containsStr(msg, "wrap") {
		t.Fatalf("error message %q should contain %q", msg, "wrap")
	}
}

func TestNoWrappedCause(t *testing.T) {
	err := apperrors.NotFound("missing")
	if err.Error() != "missing" {
		t.Fatalf("got %q, want %q", err.Error(), "missing")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
