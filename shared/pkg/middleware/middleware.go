// Package middleware provides reusable Fiber v3 middleware for all services.
// Note: Fiber v3 handler signature is func(c fiber.Ctx) error (interface, no pointer).
package middleware

import (
	"context"
	"crypto/rsa"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// Claims is the JWT payload stored in each access token.
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	JTI    string `json:"jti,omitempty"`
	jwt.RegisteredClaims
}

// JWTAuth returns a Fiber v3 middleware that validates RS256 Bearer tokens.
// publicKey is the *rsa.PublicKey used to verify the signature.
// On success it stores *Claims, userID (string), and userRole (string) in Locals.
func JWTAuth(publicKey *rsa.PublicKey) fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := c.Get(fiber.HeaderAuthorization)
		if auth == "" {
			return apperrors.Unauthorized("missing authorization header")
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return apperrors.Unauthorized("authorization header must be 'Bearer <token>'")
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, apperrors.Unauthorized("unexpected token signing method")
			}
			return publicKey, nil
		})
		if err != nil || !token.Valid {
			return apperrors.Unauthorized("invalid or expired token")
		}

		c.Locals("claims", claims)
		c.Locals("userID", claims.UserID)
		c.Locals("userRole", claims.Role)
		return c.Next()
	}
}

// SecurityHeaders adds security-related HTTP response headers to every response.
// These headers defend against XSS, clickjacking, MIME-sniffing, and information leakage.
func SecurityHeaders() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Set("Content-Security-Policy",
			"default-src 'none'; frame-ancestors 'none'")
		return c.Next()
	}
}

// OptionalJWTAuth validates a Bearer RS256 JWT if present, setting the same Locals
// as JWTAuth, but does NOT reject the request if the header is absent or invalid.
// Handlers must inspect c.Locals("userID") to determine authentication state.
func OptionalJWTAuth(publicKey *rsa.PublicKey) fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := c.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(auth, "Bearer ") {
			return c.Next()
		}
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(strings.TrimPrefix(auth, "Bearer "), claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, apperrors.Unauthorized("unexpected token signing method")
			}
			return publicKey, nil
		})
		if err != nil || !token.Valid {
			return c.Next() // invalid token → proceed as unauthenticated
		}
		c.Locals("claims", claims)
		c.Locals("userID", claims.UserID)
		c.Locals("userRole", claims.Role)
		return c.Next()
	}
}

// CORS returns a permissive CORS middleware for dev mode or a strict one for production.
// allowedOrigins is only consulted in production (devMode=false).
func CORS(devMode bool, allowedOrigins []string) fiber.Handler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}

	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")

		var allow string
		if devMode {
			allow = origin // reflect in dev
		} else if _, ok := originSet[origin]; ok {
			allow = origin
		}

		if allow != "" {
			c.Set("Access-Control-Allow-Origin", allow)
			c.Set("Vary", "Origin")
			c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
			c.Set("Access-Control-Max-Age", "86400")
		}

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

// RequireRole returns a middleware that asserts the authenticated user has one
// of the specified roles. Must be used after JWTAuth.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c fiber.Ctx) error {
		role, ok := c.Locals("userRole").(string)
		if !ok || role == "" {
			return apperrors.Unauthorized("unauthenticated")
		}
		if _, ok := allowed[role]; !ok {
			return apperrors.Forbidden("insufficient permissions")
		}
		return c.Next()
	}
}

// TokenRevocation returns a middleware that rejects requests whose access-token JTI
// appears in the revocation blacklist. It MUST run after JWTAuth (which sets Locals).
// isRevoked is a function that queries the blacklist (e.g. Redis). If isRevoked is nil
// or the token has no JTI, the check is skipped so revocation is always opt-in.
func TokenRevocation(isRevoked func(ctx context.Context, jti string) bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		if isRevoked == nil {
			return c.Next()
		}
		claims, ok := c.Locals("claims").(*Claims)
		if !ok || claims == nil || claims.JTI == "" {
			return c.Next()
		}
		if isRevoked(c.Context(), claims.JTI) {
			return apperrors.Unauthorized("token has been revoked")
		}
		return c.Next()
	}
}

// ErrorHandler is the Fiber v3 error handler that converts *AppError and
// *fiber.Error into consistent JSON responses.
// Register it via fiber.Config{ErrorHandler: middleware.ErrorHandler}.
func ErrorHandler(c fiber.Ctx, err error) error {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return c.Status(appErr.Status).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "ERROR",
				"message": fiberErr.Message,
			},
		})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    "INTERNAL_ERROR",
			"message": "an unexpected error occurred",
		},
	})
}
