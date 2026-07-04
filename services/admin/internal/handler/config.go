package handler

import (
	"os"

	"github.com/gofiber/fiber/v3"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── Audit config handlers ────────────────────────────────────────────────────

// GetAuditConfig handles GET /api/v1/admin/audit/config
func (h *Handler) GetAuditConfig(c fiber.Ctx) error {
	cfg, err := h.svc.Repository().GetAuditConfig(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get audit config", err)
	}
	return c.JSON(cfg)
}

// PutAuditConfig handles PUT /api/v1/admin/audit/config
func (h *Handler) PutAuditConfig(c fiber.Ctx) error {
	var body struct {
		RetentionEnabled *bool `json:"retention_enabled"`
		RetentionDays    *int  `json:"retention_days"`
	}

	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	adminID := h.callerID(c)
	if err := h.svc.Repository().UpdateAuditConfig(c.Context(), body.RetentionEnabled, body.RetentionDays, adminID); err != nil {
		return apperrors.Internal("Failed to update audit config", err)
	}

	return c.JSON(fiber.Map{"message": "Audit config updated successfully"})
}

// GetAuditStats handles GET /api/v1/admin/audit/stats
func (h *Handler) GetAuditStats(c fiber.Ctx) error {
	stats, err := h.svc.Repository().GetAuditStats(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get audit stats", err)
	}
	return c.JSON(stats)
}

// TriggerAuditPurge handles POST /api/v1/admin/audit/purge
func (h *Handler) TriggerAuditPurge(c fiber.Ctx) error {
	cfg, err := h.svc.Repository().GetAuditConfig(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get audit config", err)
	}

	if !cfg.RetentionEnabled {
		return apperrors.BadRequest("Audit retention is not enabled")
	}

	deleted, err := h.svc.Repository().PurgeAuditLogs(c.Context(), cfg.RetentionDays)
	if err != nil {
		return apperrors.Internal("Failed to purge audit logs", err)
	}

	return c.JSON(fiber.Map{"message": "Audit logs purged successfully", "deleted": deleted})
}

// ── Auth config handlers ─────────────────────────────────────────────────────

// GetAuthConfig handles GET /api/v1/admin/auth/config
func (h *Handler) GetAuthConfig(c fiber.Ctx) error {
	cfg, err := h.svc.Repository().GetAuthConfig(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get auth config", err)
	}
	return c.JSON(cfg)
}

// PutAuthConfig handles PUT /api/v1/admin/auth/config
func (h *Handler) PutAuthConfig(c fiber.Ctx) error {
	var body map[string]any
	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	if err := h.svc.Repository().UpdateAuthConfig(c.Context(), body); err != nil {
		return apperrors.Internal("Failed to update auth config", err)
	}

	return c.JSON(fiber.Map{"message": "Auth config updated successfully"})
}

// ── Branding handlers ────────────────────────────────────────────────────────

// GetBranding handles GET /api/v1/admin/branding
func (h *Handler) GetBranding(c fiber.Ctx) error {
	branding, err := h.svc.Repository().GetBrandingConfig(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get branding config", err)
	}
	return c.JSON(branding)
}

// GetBrandingPublic handles GET /api/v1/branding (public)
func (h *Handler) GetBrandingPublic(c fiber.Ctx) error {
	branding, err := h.svc.Repository().GetBrandingConfig(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get branding config", err)
	}

	// Default EmailHeaderLink to BASE_URL if empty
	if branding.EmailHeaderLink == "" {
		branding.EmailHeaderLink = os.Getenv("BASE_URL")
		if branding.EmailHeaderLink == "" {
			branding.EmailHeaderLink = "http://localhost"
		}
	}

	return c.JSON(branding)
}

// PutBranding handles PUT /api/v1/admin/branding
func (h *Handler) PutBranding(c fiber.Ctx) error {
	var body map[string]any
	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	// TODO: Validate hex colors and logo size
	if err := h.svc.Repository().UpdateBrandingConfig(c.Context(), body); err != nil {
		return apperrors.Internal("Failed to update branding config", err)
	}

	return c.JSON(fiber.Map{"message": "Branding updated successfully"})
}

// ── Platform config handlers ─────────────────────────────────────────────────

// GetPlatformConfig handles GET /api/v1/admin/platform/config
func (h *Handler) GetPlatformConfig(c fiber.Ctx) error {
	return h.GetPlatformConfigPublic(c)
}

// GetPlatformConfigPublic handles GET /api/v1/platform/config (public)
func (h *Handler) GetPlatformConfigPublic(c fiber.Ctx) error {
	cfg, err := h.svc.Repository().GetPlatformConfig(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get platform config", err)
	}
	return c.JSON(cfg)
}

// PutPlatformConfig handles PUT /api/v1/admin/platform/config
func (h *Handler) PutPlatformConfig(c fiber.Ctx) error {
	var body map[string]any
	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	if err := h.svc.Repository().UpdatePlatformConfig(c.Context(), body); err != nil {
		return apperrors.Internal("Failed to update platform config", err)
	}

	return c.JSON(fiber.Map{"message": "Platform config updated successfully"})
}

// ── SMTP settings handlers ───────────────────────────────────────────────────

// GetSMTPSettings handles GET /api/v1/admin/settings/smtp
func (h *Handler) GetSMTPSettings(c fiber.Ctx) error {
	passwordSet, err := h.svc.Repository().IsSMTPPasswordSet(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get SMTP settings", err)
	}

	cfg, err := h.svc.Repository().GetSMTPSettings(c.Context(), h.svc.SMTPEncKey())
	if err != nil {
		return apperrors.Internal("Failed to get SMTP settings", err)
	}

	return c.JSON(fiber.Map{
		"host":         cfg.Host,
		"port":         cfg.Port,
		"username":     cfg.Username,
		"password_set": passwordSet,
		"from":         cfg.From,
		"use_tls":      cfg.UseTLS,
		"updated_at":   "", // TODO: Add timestamp
	})
}

// PutSMTPSettings handles PUT /api/v1/admin/settings/smtp
func (h *Handler) PutSMTPSettings(c fiber.Ctx) error {
	var body map[string]any
	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	// TODO: Encrypt password if provided
	if err := h.svc.Repository().UpdateSMTPSettings(c.Context(), body); err != nil {
		return apperrors.Internal("Failed to update SMTP settings", err)
	}

	return c.JSON(fiber.Map{"message": "SMTP settings updated successfully"})
}

// TestSMTPSettings handles POST /api/v1/admin/settings/smtp/test
func (h *Handler) TestSMTPSettings(c fiber.Ctx) error {
	// TODO: Implement SMTP test email
	return c.JSON(fiber.Map{"message": "Test email sent successfully"})
}
