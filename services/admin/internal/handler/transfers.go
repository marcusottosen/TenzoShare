package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/tenzoshare/tenzoshare/services/admin/internal/repository"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// ── Transfer handlers ────────────────────────────────────────────────────────

// ListTransfers handles GET /api/v1/admin/transfers
func (h *Handler) ListTransfers(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filters := repository.TransferFilters{
		Status:  c.Query("status", "all"),
		Limit:   limit,
		Offset:  offset,
		SortBy:  c.Query("sort_by", "created_at"),
		SortDir: c.Query("sort_dir", "desc"),
	}

	transfers, total, err := h.svc.Repository().ListTransfers(c.Context(), filters)
	if err != nil {
		return apperrors.Internal("Failed to list transfers", err)
	}

	return c.JSON(fiber.Map{
		"transfers": transfers,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// GetTransfer handles GET /api/v1/admin/transfers/:id
func (h *Handler) GetTransfer(c fiber.Ctx) error {
	transferID := c.Params("id")

	transfer, err := h.svc.Repository().GetTransfer(c.Context(), transferID)
	if err != nil {
		return apperrors.NotFound("Transfer not found")
	}

	return c.JSON(transfer)
}

// RevokeTransfer handles POST /api/v1/admin/transfers/:id/revoke
func (h *Handler) RevokeTransfer(c fiber.Ctx) error {
	transferID := c.Params("id")

	if err := h.svc.Repository().RevokeTransfer(c.Context(), transferID); err != nil {
		return apperrors.Internal("Failed to revoke transfer", err)
	}

	// TODO: Publish audit event
	return c.JSON(fiber.Map{"message": "Transfer revoked successfully"})
}
