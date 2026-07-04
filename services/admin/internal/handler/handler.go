package handler
package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tenzoshare/tenzoshare/services/admin/internal/service"
)

// Handler is the base handler struct that holds dependencies.
type Handler struct {
	svc *service.Service
}

// New creates a new handler instance.
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Service returns the underlying service.
func (h *Handler) Service() *service.Service {
	return h.svc
}

// ── Helper methods ───────────────────────────────────────────────────────────

// clientIP returns the client IP resolved by Fiber from ProxyHeader.
func (h *Handler) clientIP(c fiber.Ctx) string {
	return c.IP()
}

// callerID extracts the authenticated user ID from JWT claims.
func (h *Handler) callerID(c fiber.Ctx) string {
	if userID := c.Locals("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return ""
}

// callerEmail resolves the calling user's email from JWT claims or falls back to user ID.
func (h *Handler) callerEmail(c fiber.Ctx) string {
	if email := c.Locals("email"); email != nil {
		if e, ok := email.(string); ok && e != "" {
			return e
		}
	}
	return h.callerID(c)
}
