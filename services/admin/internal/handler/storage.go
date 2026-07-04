package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/repository"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── Storage handlers ─────────────────────────────────────────────────────────

// ListStorageUsage handles GET /api/v1/admin/storage/usage
func (h *Handler) ListStorageUsage(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	sortBy := c.Query("sort_by", "total_bytes")
	sortDir := c.Query("sort_dir", "desc")

	usage, total, err := h.svc.Repository().ListStorageUsage(c.Context(), limit, offset, sortBy, sortDir)
	if err != nil {
		return apperrors.Internal("Failed to list storage usage", err)
	}

	return c.JSON(fiber.Map{
		"usage":  usage,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetStorageConfig handles GET /api/v1/admin/storage/config
func (h *Handler) GetStorageConfig(c fiber.Ctx) error {
	cfg, err := h.svc.Repository().GetStorageConfig(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get storage config", err)
	}
	return c.JSON(cfg)
}

// PutStorageConfig handles PUT /api/v1/admin/storage/config
func (h *Handler) PutStorageConfig(c fiber.Ctx) error {
	var body map[string]any
	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("Invalid request body")
	}

	adminID := h.callerID(c)
	if err := h.svc.Repository().UpdateStorageConfig(c.Context(), body, adminID); err != nil {
		return apperrors.Internal("Failed to update storage config", err)
	}

	// TODO: Publish audit event
	return c.JSON(fiber.Map{"message": "Storage config updated successfully"})
}

// ListStorageFiles handles GET /api/v1/admin/storage/files
func (h *Handler) ListStorageFiles(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filters := repository.FileFilters{
		Filter:  c.Query("filter", "all"),
		Limit:   limit,
		Offset:  offset,
		SortBy:  c.Query("sort_by", "size_bytes"),
		SortDir: c.Query("sort_dir", "desc"),
	}

	files, total, err := h.svc.Repository().ListStorageFiles(c.Context(), filters)
	if err != nil {
		return apperrors.Internal("Failed to list files", err)
	}

	return c.JSON(fiber.Map{
		"files":  files,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// DeleteFile handles DELETE /api/v1/admin/storage/files/:id
func (h *Handler) DeleteFile(c fiber.Ctx) error {
	fileID := c.Params("id")

	// Get file metadata
	file, err := h.svc.Repository().GetFile(c.Context(), fileID)
	if err != nil {
		return apperrors.NotFound("File not found")
	}

	// Soft delete
	if err := h.svc.Repository().SoftDeleteFile(c.Context(), fileID); err != nil {
		return apperrors.Internal("Failed to delete file", err)
	}

	// Log purge
	if err := h.svc.Repository().LogFilePurge(c.Context(), fileID, file.OwnerID, file.Filename,
		file.SizeBytes, "admin_manual", h.callerID(c), "admin"); err != nil {
		h.svc.Logger().Error("failed to log file purge", zap.Error(err))
	}

	return c.JSON(fiber.Map{"message": "File deleted successfully"})
}

// TriggerPurge handles POST /api/v1/admin/storage/purge
func (h *Handler) TriggerPurge(c fiber.Ctx) error {
	// TODO: Implement purge logic (move from main.go)
	return c.JSON(fiber.Map{"message": "Purge triggered", "deleted": 0})
}

// ListPurgeLog handles GET /api/v1/admin/storage/purge-log
func (h *Handler) ListPurgeLog(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	entries, total, err := h.svc.Repository().ListPurgeLog(c.Context(), limit, offset)
	if err != nil {
		return apperrors.Internal("Failed to list purge log", err)
	}

	return c.JSON(fiber.Map{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GetStorageInsights handles GET /api/v1/admin/storage/insights
func (h *Handler) GetStorageInsights(c fiber.Ctx) error {
	insights, err := h.svc.Repository().GetStorageInsights(c.Context())
	if err != nil {
		return apperrors.Internal("Failed to get storage insights", err)
	}
	return c.JSON(insights)
}
