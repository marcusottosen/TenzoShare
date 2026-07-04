package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/tenzoshare/tenzoshare/services/admin/internal/repository"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── User handlers ────────────────────────────────────────────────────────────

// ListUsers handles GET /api/v1/admin/users
func (h *Handler) ListUsers(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filters := repository.UserFilters{
		Search:  c.Query("search"),
		Role:    c.Query("role"),
		Limit:   limit,
		Offset:  offset,
		SortBy:  c.Query("sort_by", "created_at"),
		SortDir: c.Query("sort_dir", "desc"),
	}

	users, total, err := h.svc.ListUsers(c.Context(), filters)
	if err != nil {
		return apperrors.Internal("Failed to list users", err)
	}

	return c.JSON(fiber.Map{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// CreateUser handles POST /api/v1/admin/users
func (h *Handler) CreateUser(c fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	if body.Email == "" || body.Password == "" {
		return apperrors.BadRequest("Email and password are required")
	}

	if body.Role == "" {
		body.Role = "user"
	}

	userID, err := h.svc.CreateUser(c.Context(), body.Email, body.Password, body.Role,
		h.callerID(c), h.callerEmail(c), h.clientIP(c))
	if err != nil {
		if contains(err.Error(), "duplicate key") || contains(err.Error(), "unique constraint") {
			return apperrors.Conflict("Email already exists")
		}
		return apperrors.Internal("Failed to create user", err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":      userID,
		"email":   body.Email,
		"role":    body.Role,
		"message": "User created successfully",
	})
}

// UpdateUser handles PATCH /api/v1/admin/users/:id
func (h *Handler) UpdateUser(c fiber.Ctx) error {
	userID := c.Params("id")

	var body struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}

	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	if body.Role == nil && body.IsActive == nil {
		return apperrors.BadRequest("At least one field must be provided")
	}

	if err := h.svc.UpdateUser(c.Context(), userID, body.Role, body.IsActive,
		h.callerID(c), h.callerEmail(c), h.clientIP(c)); err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to update user", err)
	}

	return c.JSON(fiber.Map{"message": "User updated successfully"})
}

// DeleteUser handles DELETE /api/v1/admin/users/:id
func (h *Handler) DeleteUser(c fiber.Ctx) error {
	userID := c.Params("id")

	// Prevent deleting self
	if userID == h.callerID(c) {
		return apperrors.BadRequest("Cannot delete your own account")
	}

	if err := h.svc.DeleteUser(c.Context(), userID, h.callerID(c), h.callerEmail(c), h.clientIP(c)); err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to delete user", err)
	}

	return c.JSON(fiber.Map{"message": "User deleted successfully"})
}

// UnlockUser handles POST /api/v1/admin/users/:id/unlock
func (h *Handler) UnlockUser(c fiber.Ctx) error {
	userID := c.Params("id")

	if err := h.svc.UnlockUser(c.Context(), userID, h.callerID(c), h.callerEmail(c), h.clientIP(c)); err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to unlock user", err)
	}

	return c.JSON(fiber.Map{"message": "User unlocked successfully"})
}

// ResetUserMFA handles DELETE /api/v1/admin/users/:id/mfa
func (h *Handler) ResetUserMFA(c fiber.Ctx) error {
	userID := c.Params("id")

	if err := h.svc.ResetUserMFA(c.Context(), userID, h.callerID(c), h.callerEmail(c), h.clientIP(c)); err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to reset MFA", err)
	}

	return c.JSON(fiber.Map{"message": "MFA reset successfully"})
}

// VerifyUserEmail handles POST /api/v1/admin/users/:id/verify
func (h *Handler) VerifyUserEmail(c fiber.Ctx) error {
	userID := c.Params("id")

	if err := h.svc.VerifyUserEmail(c.Context(), userID, h.callerID(c), h.callerEmail(c), h.clientIP(c)); err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to verify email", err)
	}

	return c.JSON(fiber.Map{"message": "Email verified successfully"})
}

// ResetUserPassword handles POST /api/v1/admin/users/:id/reset-password
func (h *Handler) ResetUserPassword(c fiber.Ctx) error {
	userID := c.Params("id")

	tempPassword, err := h.svc.ResetUserPassword(c.Context(), userID, h.callerID(c), h.callerEmail(c), h.clientIP(c))
	if err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to reset password", err)
	}

	return c.JSON(fiber.Map{
		"message":            "Password reset successfully",
		"temporary_password": tempPassword,
	})
}

// SetUserPassword handles POST /api/v1/admin/users/:id/set-password
func (h *Handler) SetUserPassword(c fiber.Ctx) error {
	userID := c.Params("id")

	var body struct {
		Password string `json:"password"`
	}

	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	if body.Password == "" {
		return apperrors.BadRequest("Password is required")
	}

	if len(body.Password) < 8 {
		return apperrors.BadRequest("Password must be at least 8 characters")
	}

	if err := h.svc.SetUserPassword(c.Context(), userID, body.Password, h.callerID(c), h.callerEmail(c), h.clientIP(c)); err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to set password", err)
	}

	return c.JSON(fiber.Map{"message": "Password updated successfully"})
}

// ── User quota handlers ──────────────────────────────────────────────────────

// ListUserQuotas handles GET /api/v1/admin/quotas
func (h *Handler) ListUserQuotas(c fiber.Ctx) error {
	quotas, err := h.svc.ListUserQuotas(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to list quotas", err)
	}

	return c.JSON(fiber.Map{"quotas": quotas})
}

// GetUserQuota handles GET /api/v1/admin/users/:id/quota
func (h *Handler) GetUserQuota(c fiber.Ctx) error {
	userID := c.Params("id")

	quota, err := h.svc.GetUserQuota(c.Context(), userID)
	if err != nil {
		return apperrors.Internal("Failed to get quota", err)
	}

	if quota == nil {
		return c.JSON(fiber.Map{"quota": nil, "message": "No custom quota set"})
	}

	return c.JSON(quota)
}

// PutUserQuota handles PUT /api/v1/admin/users/:id/quota
func (h *Handler) PutUserQuota(c fiber.Ctx) error {
	userID := c.Params("id")

	var body struct {
		QuotaBytes *int64 `json:"quota_bytes"`
	}

	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	if err := h.svc.SetUserQuota(c.Context(), userID, body.QuotaBytes, h.callerID(c), h.callerEmail(c), h.clientIP(c)); err != nil {
		if contains(err.Error(), "not found") {
			return apperrors.NotFound("User not found")
		}
		return apperrors.Internal("Failed to set quota", err)
	}

	if body.QuotaBytes == nil {
		return c.JSON(fiber.Map{"message": "Quota override removed"})
	}

	return c.JSON(fiber.Map{"message": "Quota updated successfully"})
}

// ── Helper ───────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && (s == sub || len(s) >= len(sub) && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
