// Package handlers provides HTTP handlers for the audit service.
package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tenzoshare/tenzoshare/services/audit/internal/repository"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

// Handler holds the audit HTTP handlers.
type Handler struct {
	repo *repository.Repository
}

// New creates a new Handler.
func New(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

// ListEvents handles GET /api/v1/audit/events
// Query params: user_id, source, action, start, end, limit, offset
func (h *Handler) ListEvents(c fiber.Ctx) error {
	limit := 50
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v >= 0 {
		offset = v
	}
	f := repository.ListFilter{
		UserID: c.Query("user_id"),
		Source: c.Query("source"),
		Action: c.Query("action"),
		Limit:  limit,
		Offset: offset,
	}

	if s := c.Query("start"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return apperrors.BadRequest("invalid start time; use RFC3339 format")
		}
		f.StartTime = &t
	}
	if s := c.Query("end"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return apperrors.BadRequest("invalid end time; use RFC3339 format")
		}
		f.EndTime = &t
	}

	logs, total, err := h.repo.List(c.Context(), f)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"total":  total,
		"limit":  f.Limit,
		"offset": f.Offset,
		"items":  logs,
	})
}
