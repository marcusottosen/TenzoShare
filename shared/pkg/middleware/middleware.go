// Package middleware provides reusable Fiber v3 middleware for all services.
// Note: Fiber v3 handler signature is func(c fiber.Ctx) error (interface, no pointer).
package middleware

import (
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
	jwt.RegisteredClaims
}

// JWTAuth returns a Fiber v3 middleware that validates Bearer tokens.
// On success it stores *Claims, userID (string), and userRole (string) in Locals.
func JWTAuth(secret string) fiber.Handler {
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
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, apperrors.Unauthorized("unexpected token signing method")
			}
			return []byte(secret), nil
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
