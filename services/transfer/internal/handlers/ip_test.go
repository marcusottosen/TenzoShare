package handlers

// realClientIP tests live in the same package so the unexported function is accessible.

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func appWithIPEcho() *fiber.App {
	app := fiber.New()
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

func TestRealClientIP_XRealIP(t *testing.T) {
	app := appWithIPEcho()
	got := getIP(app, map[string]string{"X-Real-IP": "203.0.113.5"})
	if got != "203.0.113.5" {
		t.Errorf("got %q, want %q", got, "203.0.113.5")
	}
}

func TestRealClientIP_XForwardedFor_Single(t *testing.T) {
	app := appWithIPEcho()
	got := getIP(app, map[string]string{"X-Forwarded-For": "198.51.100.10"})
	if got != "198.51.100.10" {
		t.Errorf("got %q, want %q", got, "198.51.100.10")
	}
}

func TestRealClientIP_XForwardedFor_MultipleHops(t *testing.T) {
	// Only the leftmost (client) IP should be returned
	app := appWithIPEcho()
	got := getIP(app, map[string]string{"X-Forwarded-For": "198.51.100.10, 10.0.0.1, 172.16.0.1"})
	if got != "198.51.100.10" {
		t.Errorf("got %q, want %q", got, "198.51.100.10")
	}
}

func TestRealClientIP_XRealIP_TakesPrecedence(t *testing.T) {
	// X-Real-IP should win over X-Forwarded-For
	app := appWithIPEcho()
	got := getIP(app, map[string]string{
		"X-Real-IP":       "203.0.113.5",
		"X-Forwarded-For": "198.51.100.10",
	})
	if got != "203.0.113.5" {
		t.Errorf("got %q, want %q (X-Real-IP should take precedence)", got, "203.0.113.5")
	}
}

func TestRealClientIP_WhitespaceTrimmed(t *testing.T) {
	app := appWithIPEcho()
	got := getIP(app, map[string]string{"X-Real-IP": "  203.0.113.5  "})
	if got != "203.0.113.5" {
		t.Errorf("got %q, want trimmed %q", got, "203.0.113.5")
	}
}

func TestRealClientIP_NoHeaders_FallsBackToConnIP(t *testing.T) {
	app := appWithIPEcho()
	got := getIP(app, nil)
	// Connection IP is 192.0.2.1 in Fiber's test harness; just assert non-empty
	if got == "" {
		t.Error("expected non-empty fallback IP")
	}
}
