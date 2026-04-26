package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func signRS256(t *testing.T, key *rsa.PrivateKey, claims jwt.Claims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func newTestApp(handler fiber.Handler, mws ...fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler})
	for _, mw := range mws {
		app.Use(mw)
	}
	app.Get("/test", handler)
	return app
}

func get(app *fiber.App, path, authHeader string) int {
	req := httptest.NewRequest("GET", path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) //nolint:errcheck
	return resp.StatusCode
}

// ── JWTAuth tests ─────────────────────────────────────────────────────────────

func TestJWTAuth_ValidToken(t *testing.T) {
	key := generateRSAKey(t)
	now := time.Now()
	claims := &middleware.Claims{
		UserID: "user-1",
		Email:  "test@example.com",
		Role:   "user",
		JTI:    uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	tok := signRS256(t, key, claims)

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey))

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 got %d", code)
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	key := generateRSAKey(t)
	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey))

	code := get(app, "/test", "")
	if code != 401 {
		t.Errorf("expected 401 got %d", code)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	key := generateRSAKey(t)
	past := time.Now().Add(-1 * time.Hour)
	claims := &middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(past),
		},
	}
	tok := signRS256(t, key, claims)

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey))

	code := get(app, "/test", "Bearer "+tok)
	if code != 401 {
		t.Errorf("expected 401 got %d", code)
	}
}

func TestJWTAuth_WrongSigningMethod(t *testing.T) {
	key := generateRSAKey(t)
	verifyKey := generateRSAKey(t) // different key

	now := time.Now()
	claims := &middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}
	tok := signRS256(t, key, claims) // signed with key, verified with verifyKey → fail

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&verifyKey.PublicKey))

	code := get(app, "/test", "Bearer "+tok)
	if code != 401 {
		t.Errorf("expected 401 got %d", code)
	}
}

func TestJWTAuth_WrongScheme(t *testing.T) {
	key := generateRSAKey(t)
	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey))

	code := get(app, "/test", "Basic dXNlcjpwYXNz")
	if code != 401 {
		t.Errorf("expected 401 got %d", code)
	}
}

// ── RequireRole tests ─────────────────────────────────────────────────────────

func validToken(t *testing.T, key *rsa.PrivateKey, role string) string {
	t.Helper()
	now := time.Now()
	claims := &middleware.Claims{
		UserID: "user-1",
		Role:   role,
		JTI:    uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return signRS256(t, key, claims)
}

func TestRequireRole_Allowed(t *testing.T) {
	key := generateRSAKey(t)
	tok := validToken(t, key, "admin")

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), middleware.RequireRole("admin"))

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 got %d", code)
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	key := generateRSAKey(t)
	tok := validToken(t, key, "user")

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), middleware.RequireRole("admin"))

	code := get(app, "/test", "Bearer "+tok)
	if code != 403 {
		t.Errorf("expected 403 got %d", code)
	}
}

// ── TokenRevocation tests ─────────────────────────────────────────────────────

func TestTokenRevocation_NotRevoked(t *testing.T) {
	key := generateRSAKey(t)
	jti := uuid.NewString()
	now := time.Now()
	claims := &middleware.Claims{
		UserID: "user-1",
		Role:   "user",
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	tok := signRS256(t, key, claims)

	revocationCheck := middleware.TokenRevocation(func(_ context.Context, _ string) bool {
		return false // nothing is revoked
	})

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), revocationCheck)

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 got %d", code)
	}
}

func TestTokenRevocation_Revoked(t *testing.T) {
	key := generateRSAKey(t)
	jti := uuid.NewString()
	now := time.Now()
	claims := &middleware.Claims{
		UserID: "user-1",
		Role:   "user",
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	tok := signRS256(t, key, claims)

	revocationCheck := middleware.TokenRevocation(func(_ context.Context, tokenJTI string) bool {
		return tokenJTI == jti // simulate this specific JTI being revoked
	})

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), revocationCheck)

	code := get(app, "/test", "Bearer "+tok)
	if code != 401 {
		t.Errorf("expected 401 got %d", code)
	}
}

func TestTokenRevocation_NilChecker(t *testing.T) {
	key := generateRSAKey(t)
	tok := validToken(t, key, "user")

	// nil isRevoked function — should always pass
	revocationCheck := middleware.TokenRevocation(nil)

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), revocationCheck)

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 got %d", code)
	}
}

// ── OptionalJWTAuth tests ─────────────────────────────────────────────────────

func TestOptionalJWTAuth_ValidToken_SetsLocals(t *testing.T) {
	key := generateRSAKey(t)
	tok := validToken(t, key, "user")

	app := newTestApp(func(c fiber.Ctx) error {
		uid, _ := c.Locals("userID").(string)
		if uid == "" {
			return c.SendStatus(401)
		}
		return c.SendStatus(200)
	}, middleware.OptionalJWTAuth(&key.PublicKey))

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 got %d", code)
	}
}

func TestOptionalJWTAuth_NoToken_StillPasses(t *testing.T) {
	key := generateRSAKey(t)

	app := newTestApp(func(c fiber.Ctx) error {
		// userID should be empty — no token provided
		uid, _ := c.Locals("userID").(string)
		if uid != "" {
			return c.SendStatus(500)
		}
		return c.SendStatus(200)
	}, middleware.OptionalJWTAuth(&key.PublicKey))

	code := get(app, "/test", "")
	if code != 200 {
		t.Errorf("expected 200 (unauthenticated) got %d", code)
	}
}

func TestOptionalJWTAuth_InvalidToken_StillPasses(t *testing.T) {
	key := generateRSAKey(t)

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.OptionalJWTAuth(&key.PublicKey))

	// Bad token — should be treated as unauthenticated, not 401
	code := get(app, "/test", "Bearer badtoken.notvalid.jwt")
	if code != 200 {
		t.Errorf("expected 200 (invalid token gracefully ignored) got %d", code)
	}
}

func TestOptionalJWTAuth_WrongKey_StillPasses(t *testing.T) {
	signingKey := generateRSAKey(t)
	verifyKey := generateRSAKey(t)
	tok := validToken(t, signingKey, "user")

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.OptionalJWTAuth(&verifyKey.PublicKey))

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 (wrong key treated as unauthenticated) got %d", code)
	}
}

// ── SecurityHeaders tests ─────────────────────────────────────────────────────

func TestSecurityHeaders_Present(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.SecurityHeaders())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) //nolint:errcheck

	headers := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-Xss-Protection":       "1; mode=block",
	}
	for h, want := range headers {
		got := resp.Header.Get(h)
		if got != want {
			t.Errorf("header %s = %q, want %q", h, got, want)
		}
	}
}
