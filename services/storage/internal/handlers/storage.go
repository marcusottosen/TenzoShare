package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/tenzoshare/tenzoshare/services/storage/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/storage/internal/repository"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	sharedStorage "github.com/tenzoshare/tenzoshare/shared/pkg/storage"
)

type Handler struct {
	repo    *repository.FileRepository
	backend sharedStorage.Backend
}

func New(repo *repository.FileRepository, backend sharedStorage.Backend) *Handler {
	return &Handler{repo: repo, backend: backend}
}

// Upload handles multipart file uploads.
func (h *Handler) Upload(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return apperrors.BadRequest("missing file field")
	}

	f, err := fileHeader.Open()
	if err != nil {
		return apperrors.Internal("open uploaded file", err)
	}
	defer f.Close()

	objectKey := fmt.Sprintf("uploads/%s/%s/%s", ownerID, uuid.New().String(), fileHeader.Filename)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.backend.Upload(c.Context(), objectKey, f, fileHeader.Size, contentType); err != nil {
		return apperrors.Internal("upload to object store", err)
	}

	record, err := h.repo.Create(c.Context(), ownerID, objectKey, fileHeader.Filename, contentType, fileHeader.Size)
	if err != nil {
		// best-effort cleanup
		_ = h.backend.Delete(c.Context(), objectKey)
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fileResponse(record))
}

// GetFile returns file metadata.
func (h *Handler) GetFile(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	id := c.Params("id")

	file, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return err
	}

	// only the owner (or admin) can access
	role, _ := c.Locals("userRole").(string)
	if file.OwnerID != ownerID && role != "admin" {
		return apperrors.Forbidden("access denied")
	}

	return c.JSON(fileResponse(file))
}

// ListFiles returns all files owned by the authenticated user.
func (h *Handler) ListFiles(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	limit := 50
	offset := 0
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

	files, err := h.repo.ListByOwner(c.Context(), ownerID, limit, offset)
	if err != nil {
		return err
	}

	items := make([]fiber.Map, 0, len(files))
	for _, f := range files {
		items = append(items, fileResponse(f))
	}
	return c.JSON(fiber.Map{"files": items, "limit": limit, "offset": offset})
}

// DeleteFile soft-deletes the metadata and removes the object from storage.
func (h *Handler) DeleteFile(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	id := c.Params("id")

	file, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return err
	}

	role, _ := c.Locals("userRole").(string)
	if file.OwnerID != ownerID && role != "admin" {
		return apperrors.Forbidden("access denied")
	}

	if err := h.repo.SoftDelete(c.Context(), id, file.OwnerID); err != nil {
		return err
	}
	// fire-and-forget object deletion; a background job can clean up orphans
	go func() { _ = h.backend.Delete(context.Background(), file.ObjectKey) }() //nolint:errcheck

	return c.SendStatus(fiber.StatusNoContent)
}

// PresignURL returns a short-lived download URL.
func (h *Handler) PresignURL(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	id := c.Params("id")

	file, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return err
	}

	role, _ := c.Locals("userRole").(string)
	if file.OwnerID != ownerID && role != "admin" {
		return apperrors.Forbidden("access denied")
	}

	url, err := h.backend.GetPresignedURL(c.Context(), file.ObjectKey, file.Filename, 15*time.Minute)
	if err != nil {
		return apperrors.Internal("generate presigned url", err)
	}

	return c.JSON(fiber.Map{"url": url, "expires_in": 900})
}

func fileResponse(f *domain.File) fiber.Map {
	return fiber.Map{
		"id":           f.ID,
		"owner_id":     f.OwnerID,
		"filename":     f.Filename,
		"content_type": f.ContentType,
		"size_bytes":   f.SizeBytes,
		"created_at":   f.CreatedAt,
	}
}
