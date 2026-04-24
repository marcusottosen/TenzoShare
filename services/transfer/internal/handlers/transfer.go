package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/go-playground/validator/v10"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/service"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

type Handler struct {
	svc      *service.TransferService
	validate *validator.Validate
}

func New(svc *service.TransferService) *Handler {
	return &Handler{svc: svc, validate: validator.New()}
}

type createRequest struct {
	FileIDs        []string `json:"file_ids" validate:"required,min=1,dive,uuid4"`
	RecipientEmail string   `json:"recipient_email" validate:"omitempty,email"`
	Password       string   `json:"password"`
	MaxDownloads   int      `json:"max_downloads" validate:"min=0"`
	ExpiresInHours int      `json:"expires_in_hours" validate:"min=0"`
}

// Create POST /api/v1/transfers
func (h *Handler) Create(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	var req createRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid JSON")
	}
	if err := h.validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	var expiry time.Duration
	if req.ExpiresInHours > 0 {
		expiry = time.Duration(req.ExpiresInHours) * time.Hour
	}

	result, err := h.svc.Create(c.Context(), service.CreateParams{
		OwnerID:        ownerID,
		FileIDs:        req.FileIDs,
		RecipientEmail: req.RecipientEmail,
		Password:       req.Password,
		MaxDownloads:   req.MaxDownloads,
		ExpiresIn:      expiry,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(transferResponse(result.Transfer, result.FileIDs))
}

// Get GET /api/v1/transfers/:id
func (h *Handler) Get(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	id := c.Params("id")

	t, fileIDs, err := h.svc.Get(c.Context(), id, ownerID)
	if err != nil {
		return err
	}

	return c.JSON(transferResponse(t, fileIDs))
}

// List GET /api/v1/transfers
func (h *Handler) List(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	limit, offset := 50, 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	transfers, err := h.svc.List(c.Context(), ownerID, limit, offset)
	if err != nil {
		return err
	}

	items := make([]fiber.Map, 0, len(transfers))
	for _, t := range transfers {
		items = append(items, transferResponse(t, nil))
	}
	return c.JSON(fiber.Map{"transfers": items, "limit": limit, "offset": offset})
}

// Revoke DELETE /api/v1/transfers/:id
func (h *Handler) Revoke(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	id := c.Params("id")

	if err := h.svc.Revoke(c.Context(), id, ownerID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Access is the public (unauthenticated) download-info endpoint.
// GET /api/v1/t/:slug  — optionally with ?password=
func (h *Handler) Access(c fiber.Ctx) error {
	slug := c.Params("slug")
	password := c.Query("password")

	result, err := h.svc.Access(c.Context(), service.AccessParams{
		Slug:     slug,
		Password: password,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"transfer": transferResponse(result.Transfer, result.FileIDs),
	})
}

func transferResponse(t *domain.Transfer, fileIDs []string) fiber.Map {
	m := fiber.Map{
		"id":              t.ID,
		"owner_id":        t.OwnerID,
		"slug":            t.Slug,
		"max_downloads":   t.MaxDownloads,
		"download_count":  t.DownloadCount,
		"is_revoked":      t.IsRevoked,
		"has_password":    t.PasswordHash != "",
		"created_at":      t.CreatedAt,
	}
	if t.RecipientEmail != "" {
		m["recipient_email"] = t.RecipientEmail
	}
	if t.ExpiresAt != nil {
		m["expires_at"] = t.ExpiresAt
	}
	if fileIDs != nil {
		m["file_ids"] = fileIDs
	}
	return m
}
