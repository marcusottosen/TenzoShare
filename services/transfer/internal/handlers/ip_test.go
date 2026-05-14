package handlers

// realClientIP tests live in the same package so the unexported function is accessible.

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// appWithIPEcho creates a Fiber app that returns the result of realClientIP.
// proxyHeader configures fiber.Config.ProxyHeader, matching the production setup.
// The test harness uses 0.0.0.0 as the remote address, so that IP is added to
// the Proxies trust list (production uses Private: true for Docker 172.x.x.x).
func appWithIPEcho(proxyHeader string) *fiber.App {
	app := fiber.New(fiber.Config{
		ProxyHeader: proxyHeader,
		TrustProxy:  true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: []string{"0.0.0.0"}, // Fiber test harness remote address
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

// With ProxyHeader:"X-Real-IP" (production config), c.IP() returns X-Real-IP.
func TestRealClientIP_XRealIP(t *testing.T) {
	app := appWithIPEcho("X-Real-IP")
	got := getIP(app, map[string]string{"X-Real-IP": "203.0.113.5"})
	if got != "203.0.113.5" {
		t.Errorf("got %q, want %q", got, "203.0.113.5")
	}
}

// X-Forwarded-For is not the configured ProxyHeader, so it must be ignored.
func TestRealClientIP_XForwardedFor_Ignored(t *testing.T) {
	// Use a separate app with no ProxyHeader — X-Forwarded-For must never be trusted.
	app := fiber.New(fiber.Config{TrustProxy: true, TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}}})
	app.Get("/ip", func(c fiber.Ctx) error { return c.SendString(realClientIP(c)) })
	got := getIP(app, map[string]string{"X-Forwarded-For": "198.51.100.10"})
	// Should return connection IP, not the header value
	if got == "198.51.100.10" {
		t.Error("X-Forwarded-For must not be trusted — IP spoofing risk")
	}
}

// Without ProxyHeader set, c.IP() returns the connection IP regardless of headers.
func TestRealClientIP_NoProxyHeader_IgnoresXRealIP(t *testing.T) {
	// TrustProxy is true but ProxyHeader is empty — X-Real-IP must still be ignored.
	app := fiber.New(fiber.Config{TrustProxy: true, TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}}})
	app.Get("/ip", func(c fiber.Ctx) error { return c.SendString(realClientIP(c)) })
	got := getIP(app, map[string]string{"X-Real-IP": "203.0.113.5"})
	// Without ProxyHeader config, arbitrary headers must not influence c.IP()
	if got == "203.0.113.5" {
		t.Error("X-Real-IP must not be trusted without ProxyHeader config")
	}
}

// No headers — c.IP() returns the connection remote address or empty string
// (Fiber returns "" when ProxyHeader is configured but the header is absent).
func TestRealClientIP_NoHeaders_FallsBackToConnIP(t *testing.T) {
	app := appWithIPEcho("X-Real-IP")
	got := getIP(app, nil)
	// Must never return a spoofed external IP; empty or connection IP are both acceptable.
	if got == "198.51.100.10" || got == "203.0.113.5" {
		t.Errorf("unexpected spoofed IP in fallback: %q", got)
	}
}
