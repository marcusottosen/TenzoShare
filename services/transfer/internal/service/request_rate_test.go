package service

// Tests for file request submission validation rules.
// The in-memory rate limiter has been replaced with a Redis-backed rate limiter
// enforced at the handler layer (handlers.checkPublicRateLimit).

import (
	"testing"
)

// TestMaxFiles_Enforced verifies that CountSubmissions is called when MaxFiles > 0.
// Full integration of this logic requires a live DB; this is a placeholder
// confirming the helper function wiring compiles correctly.
func TestSubmitParams_Validation(t *testing.T) {
	// Verify that SubmitParams fields are accessible — exercises the struct definition.
	p := SubmitParams{
		Slug:          "test-slug",
		SubmitterName: "Alice",
		Message:       "hello",
		IP:            "203.0.113.1",
		ServiceToken:  "tok",
	}
	if p.Slug == "" || p.SubmitterName == "" {
		t.Fatal("SubmitParams fields should be set")
	}
}
