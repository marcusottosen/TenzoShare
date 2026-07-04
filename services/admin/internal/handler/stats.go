package handler

import (
	"github.com/gofiber/fiber/v3"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── Stats handlers ───────────────────────────────────────────────────────────

// GetStats handles GET /api/v1/admin/stats
func (h *Handler) GetStats(c fiber.Ctx) error {
	stats, err := h.svc.Repository().GetSystemStats(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get stats", err)
	}
	return c.JSON(stats)
}

// GetSystemHealth handles GET /api/v1/admin/system/health
func (h *Handler) GetSystemHealth(c fiber.Ctx) error {
	// TODO: Implement service health checks
	return c.JSON(fiber.Map{
		"status": "ok",
		"services": []fiber.Map{
			{"name": "admin", "status": "healthy", "latency_ms": 0},
		},
	})
}

// GetSystemMetrics handles GET /api/v1/admin/system/metrics
func (h *Handler) GetSystemMetrics(c fiber.Ctx) error {
	// TODO: Implement system metrics (disk, CPU, memory)
	return c.JSON(fiber.Map{
		"storage": fiber.Map{
			"total_bytes":     0,
			"used_bytes":      0,
			"available_bytes": 0,
			"used_percent":    0,
		},
		"memory": fiber.Map{
			"total_bytes":     0,
			"used_bytes":      0,
			"available_bytes": 0,
			"used_percent":    0,
		},
		"cpu": fiber.Map{
			"used_percent": 0,
			"cores":        0,
		},
	})
}
