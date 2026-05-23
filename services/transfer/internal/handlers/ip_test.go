package handlers

// realClientIP tests live in the same package so the unexported function is accessible.

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// appWithIPEcho builds a test Fiber app that mirrors the production config:
// TrustProxy enabled for Docker-internal IPs, ProxyHeader: "X-Real-IP".
// The test framework uses 0.0.0.0 as the remote address, which we add to
// the trusted proxies list so the header is honoured in tests.
func appWithIPEcho() *fiber.App {
	app := fiber.New(fiber.Config{
		ProxyHeader: "X-Real-IP",
		TrustProxy:  true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Private:  true,
			Loopback: true,
			// Fiber test harness uses 0.0.0.0 as the remote address.
			Proxies: []string{"0.0.0.0"},
		},
	})
	app.Get("/ip", func(c fiber.Ctx) error {
		return c.SendString(realClientIP(c))
	})
	return app
}

func getIP(app *fiber.App, headers map[string]string) string {
	req := httptest.NewRequest("GET", "/ip", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}

// TestRealClientIP_XRealIP verifies that X-Real-IP (set by Traefik) is returned.
func TestRealClientIP_XRealIP(t *testing.T) {
	app := appWithIPEcho()
	got := getIP(app, map[string]string{"X-Real-IP": "203.0.113.5"})
	if got != "203.0.113.5" {
		t.Errorf("got %q, want %q", got, "203.0.113.5")
	}
}

// TestRealClientIP_XForwardedFor_Ignored verifies that X-Forwarded-For is NOT
// trusted when X-Real-IP is absent. Since Traefik always sets X-Real-IP, the
// absence here means there is no trusted proxy — so the raw connection IP is used,
// not the forged X-Forwarded-For value.
func TestRealClientIP_XForwardedFor_Ignored(t *testing.T) {
	app := appWithIPEcho()
	got := getIP(app, map[string]string{"X-Forwarded-For": "198.51.100.10"})
	// Should NOT return the forged XFF value — it is ignored entirely.
	if got == "198.51.100.10" {
		t.Error("X-Forwarded-For must not be trusted — it can be forged by clients")
	}
}

// TestRealClientIP_XRealIP_TakesPrecedence verifies that when both headers are
// present, only X-Real-IP is used (X-Forwarded-For is ignored).
func TestRealClientIP_XRealIP_TakesPrecedence(t *testing.T) {
	app := appWithIPEcho()
	got := getIP(app, map[string]string{
		"X-Real-IP":       "203.0.113.5",
		"X-Forwarded-For": "198.51.100.10",
	})
	if got != "203.0.113.5" {
		t.Errorf("got %q, want %q (only X-Real-IP should be used)", got, "203.0.113.5")
	}
}

// TestRealClientIP_NoHeaders returns whatever c.IP() falls back to when no
// proxy header is present (typically the connection IP from the test harness).
func TestRealClientIP_NoHeaders_FallsBackToConnIP(t *testing.T) {
	app := appWithIPEcho()
	// With no X-Real-IP header the result is the connection address.
	// Just assert the call does not panic; the exact value is harness-dependent.
	_ = getIP(app, nil)
}
