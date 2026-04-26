package service

// Tests for the in-memory sliding-window rate limiter used by the public upload endpoint.
// These tests live in package service so they can access the unexported allowIP method.

import (
	"testing"
	"time"
)

func newRateSvc() *RequestService {
	return &RequestService{
		rateWindow: make(map[string][]time.Time),
	}
}

func TestAllowIP_UnderLimit(t *testing.T) {
	svc := newRateSvc()
	ip := "203.0.113.1"
	const limit = 3

	for i := 0; i < limit; i++ {
		if !svc.allowIP(ip, limit, time.Hour) {
			t.Fatalf("request %d should be allowed (under limit %d)", i+1, limit)
		}
	}
}

func TestAllowIP_AtLimit_DeniesNext(t *testing.T) {
	svc := newRateSvc()
	ip := "203.0.113.2"
	const limit = 3

	for i := 0; i < limit; i++ {
		svc.allowIP(ip, limit, time.Hour) //nolint:errcheck — consume the limit
	}
	if svc.allowIP(ip, limit, time.Hour) {
		t.Fatal("request beyond the limit should be denied")
	}
}

func TestAllowIP_DifferentIPs_IndependentBuckets(t *testing.T) {
	svc := newRateSvc()
	const limit = 2

	// Exhaust limit for ip1
	svc.allowIP("10.0.0.1", limit, time.Hour)
	svc.allowIP("10.0.0.1", limit, time.Hour)
	if svc.allowIP("10.0.0.1", limit, time.Hour) {
		t.Fatal("ip1 should be rate-limited")
	}

	// ip2 should still be allowed
	if !svc.allowIP("10.0.0.2", limit, time.Hour) {
		t.Fatal("ip2 should not be rate-limited — it's a separate bucket")
	}
}

func TestAllowIP_OldTimestampsExpireFromWindow(t *testing.T) {
	svc := newRateSvc()
	ip := "203.0.113.3"
	const limit = 2

	// Manually insert old timestamps outside the 1-second window
	svc.rateMu.Lock()
	svc.rateWindow[ip] = []time.Time{
		time.Now().Add(-2 * time.Second),
		time.Now().Add(-3 * time.Second),
	}
	svc.rateMu.Unlock()

	// These stale entries should be swept, so the IP has room again
	if !svc.allowIP(ip, limit, time.Second) {
		t.Fatal("stale entries should have expired; request should be allowed")
	}
}

func TestAllowIP_ZeroLimitDeniesAll(t *testing.T) {
	svc := newRateSvc()
	if svc.allowIP("203.0.113.4", 0, time.Hour) {
		t.Fatal("limit=0 should deny all requests")
	}
}
