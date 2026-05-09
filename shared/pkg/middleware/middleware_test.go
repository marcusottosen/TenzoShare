package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
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

// ── ErrorHandler tests ────────────────────────────────────────────────────────

func testErrorHandlerApp() *fiber.App {
	return fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler})
}

func getErrorBody(app *fiber.App, path string, causeErr error) (int, map[string]interface{}) {
	app.Get(path, func(c fiber.Ctx) error {
		return causeErr
	})
	req := httptest.NewRequest("GET", path, nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	json.Unmarshal(body, &m) //nolint:errcheck
	return resp.StatusCode, m
}

func TestErrorHandler_AppError_ReturnsCorrectStatus(t *testing.T) {
	app := testErrorHandlerApp()
	status, body := getErrorBody(app, "/not-found", apperrors.NotFound("thing not found"))
	if status != 404 {
		t.Errorf("expected 404, got %d", status)
	}
	errMap, _ := body["error"].(map[string]interface{})
	if errMap == nil {
		t.Fatal("expected 'error' key in response body")
	}
	if errMap["code"] != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %v", errMap["code"])
	}
}

func TestErrorHandler_UnauthorizedError_Returns401(t *testing.T) {
	app := testErrorHandlerApp()
	status, _ := getErrorBody(app, "/unauth", apperrors.Unauthorized("invalid token"))
	if status != 401 {
		t.Errorf("expected 401, got %d", status)
	}
}

func TestErrorHandler_FiberError_ReturnsCorrectStatus(t *testing.T) {
	app := testErrorHandlerApp()
	status, body := getErrorBody(app, "/fiber-err", fiber.NewError(fiber.StatusMethodNotAllowed, "method not allowed"))
	if status != 405 {
		t.Errorf("expected 405, got %d", status)
	}
	errMap, _ := body["error"].(map[string]interface{})
	if errMap == nil {
		t.Fatal("expected 'error' key in response body")
	}
}

func TestErrorHandler_UnknownError_Returns500(t *testing.T) {
	app := testErrorHandlerApp()
	status, body := getErrorBody(app, "/unknown", errors.New("something went wrong"))
	if status != 500 {
		t.Errorf("expected 500, got %d", status)
	}
	errMap, _ := body["error"].(map[string]interface{})
	if errMap == nil {
		t.Fatal("expected 'error' key in response body")
	}
	if errMap["code"] != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR, got %v", errMap["code"])
	}
}

// ── CORS tests ────────────────────────────────────────────────────────────────

func newCORSApp(devMode bool, origins []string) *fiber.App {
	app := fiber.New()
	app.Use(middleware.CORS(devMode, origins))
	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(200) })
	app.Options("/test", func(c fiber.Ctx) error { return c.SendStatus(204) })
	return app
}

func corsRequest(app *fiber.App, method, path, origin string) (int, string) {
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) //nolint:errcheck
	return resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin")
}

func TestCORS_DevMode_ReflectsOrigin(t *testing.T) {
	app := newCORSApp(true, nil)
	_, allowOrigin := corsRequest(app, "GET", "/test", "https://example.com")
	if allowOrigin != "https://example.com" {
		t.Errorf("expected reflected origin in dev mode, got %q", allowOrigin)
	}
}

func TestCORS_DevMode_NoOriginHeader_NoACHeader(t *testing.T) {
	app := newCORSApp(true, nil)
	_, allowOrigin := corsRequest(app, "GET", "/test", "")
	if allowOrigin != "" {
		t.Errorf("expected empty ACAO header with no Origin, got %q", allowOrigin)
	}
}

func TestCORS_Prod_AllowedOrigin_SetsHeader(t *testing.T) {
	app := newCORSApp(false, []string{"https://allowed.example.com"})
	_, allowOrigin := corsRequest(app, "GET", "/test", "https://allowed.example.com")
	if allowOrigin != "https://allowed.example.com" {
		t.Errorf("expected allowed origin, got %q", allowOrigin)
	}
}

func TestCORS_Prod_DisallowedOrigin_NoHeader(t *testing.T) {
	app := newCORSApp(false, []string{"https://allowed.example.com"})
	_, allowOrigin := corsRequest(app, "GET", "/test", "https://evil.example.com")
	if allowOrigin != "" {
		t.Errorf("expected no ACAO header for disallowed origin, got %q", allowOrigin)
	}
}

func TestCORS_Preflight_Returns204(t *testing.T) {
	app := newCORSApp(true, nil)
	status, _ := corsRequest(app, "OPTIONS", "/test", "https://example.com")
	if status != 204 {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", status)
	}
}

// ── RequireRole multiple roles ────────────────────────────────────────────────

func TestRequireRole_MultipleRoles_AdminAllowed(t *testing.T) {
	key := generateRSAKey(t)
	tok := validToken(t, key, "admin")

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), middleware.RequireRole("user", "admin"))

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 for admin with multi-role middleware, got %d", code)
	}
}

func TestRequireRole_MultipleRoles_UserAllowed(t *testing.T) {
	key := generateRSAKey(t)
	tok := validToken(t, key, "user")

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), middleware.RequireRole("user", "admin"))

	code := get(app, "/test", "Bearer "+tok)
	if code != 200 {
		t.Errorf("expected 200 for user with multi-role middleware, got %d", code)
	}
}

func TestRequireRole_MultipleRoles_GuestForbidden(t *testing.T) {
	key := generateRSAKey(t)
	tok := validToken(t, key, "guest")

	app := newTestApp(func(c fiber.Ctx) error {
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey), middleware.RequireRole("user", "admin"))

	code := get(app, "/test", "Bearer "+tok)
	if code != 403 {
		t.Errorf("expected 403 for guest, got %d", code)
	}
}

// ── JWTAuth sets Locals ───────────────────────────────────────────────────────

func TestJWTAuth_ValidToken_SetsLocals(t *testing.T) {
	key := generateRSAKey(t)
	userID := uuid.NewString()
	now := time.Now()
	claims := &middleware.Claims{
		UserID: userID,
		Email:  "locals@example.com",
		Role:   "user",
		JTI:    uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	tok := signRS256(t, key, claims)

	app := newTestApp(func(c fiber.Ctx) error {
		uid, _ := c.Locals("userID").(string)
		role, _ := c.Locals("userRole").(string)
		cl, _ := c.Locals("claims").(*middleware.Claims)
		if uid != userID || role != "user" || cl == nil || cl.Email != "locals@example.com" {
			return c.Status(400).SendString("locals mismatch: uid=" + uid + " role=" + role)
		}
		return c.SendStatus(200)
	}, middleware.JWTAuth(&key.PublicKey))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 and correct locals, got %d — body: %s", resp.StatusCode, string(body))
	}
}
