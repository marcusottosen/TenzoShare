package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/service"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

// realClientIP returns the real client IP from X-Real-IP or X-Forwarded-For,
// falling back to the raw connection address. This is needed because requests
// arrive via Traefik or an nginx proxy which masks the original IP.
func realClientIP(c fiber.Ctx) string {
	if ip := c.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	return c.IP()
}

// --- Auth-required endpoints ---

type createRequestBody struct {
	Name            string   `json:"name"             validate:"required,min=1,max=200"`
	Description     string   `json:"description"      validate:"max=1000"`
	AllowedTypes    string   `json:"allowed_types"    validate:"max=500"`
	MaxSizeMB       int      `json:"max_size_mb"`
	MaxFiles        int      `json:"max_files"`
	ExpiresInHrs    int      `json:"expires_in_hours" validate:"required,min=1,max=8760"`
	RecipientEmails []string `json:"recipient_emails" validate:"omitempty,max=20,dive,email"`
	NotifyOnUpload  bool     `json:"notify_on_upload"`
}

// CreateFileRequest POST /api/v1/requests
func (h *Handler) CreateFileRequest(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(*middleware.Claims)
	if !ok || claims == nil {
		return apperrors.Unauthorized("unauthenticated")
	}

	var req createRequestBody
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	}

	fr, err := h.requestSvc.Create(c.Context(), service.CreateRequestParams{
		OwnerID:        claims.UserID,
		Name:           req.Name,
		Description:    req.Description,
		AllowedTypes:   req.AllowedTypes,
		MaxSizeMB:      req.MaxSizeMB,
		MaxFiles:       req.MaxFiles,
		ExpiresInHrs:   req.ExpiresInHrs,
		NotifyEmails:   strings.Join(req.RecipientEmails, ","),
		NotifyOnUpload: req.NotifyOnUpload,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fileRequestResponse(fr))
}

// ListFileRequests GET /api/v1/requests
func (h *Handler) ListFileRequests(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(*middleware.Claims)
	if !ok || claims == nil {
		return apperrors.Unauthorized("unauthenticated")
	}

	list, err := h.requestSvc.List(c.Context(), claims.UserID, 50, 0)
	if err != nil {
		return err
	}

	out := make([]fiber.Map, 0, len(list))
	for _, r := range list {
		out = append(out, fileRequestResponse(r))
	}
	return c.JSON(fiber.Map{"requests": out})
}

// GetFileRequest GET /api/v1/requests/:id
func (h *Handler) GetFileRequest(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(*middleware.Claims)
	if !ok || claims == nil {
		return apperrors.Unauthorized("unauthenticated")
	}

	id := c.Params("id")
	req, subs, err := h.requestSvc.Get(c.Context(), id, claims.UserID)
	if err != nil {
		return err
	}

	subList := make([]fiber.Map, 0, len(subs))
	for _, s := range subs {
		subList = append(subList, submissionResponse(s))
	}

	resp := fileRequestResponse(req)
	resp["submissions"] = subList
	return c.JSON(resp)
}

// DeactivateFileRequest DELETE /api/v1/requests/:id
func (h *Handler) DeactivateFileRequest(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(*middleware.Claims)
	if !ok || claims == nil {
		return apperrors.Unauthorized("unauthenticated")
	}

	id := c.Params("id")
	if err := h.requestSvc.Deactivate(c.Context(), id, claims.UserID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateRequestRecipients PATCH /api/v1/requests/:id/recipients
// Replaces the invite-email list for a file request. Owner only.
func (h *Handler) UpdateRequestRecipients(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(*middleware.Claims)
	if !ok || claims == nil {
		return apperrors.Unauthorized("unauthenticated")
	}

	var body struct {
		Emails []string `json:"emails" validate:"omitempty,max=20,dive,email"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("invalid JSON")
	}
	if err := h.validate.Struct(body); err != nil {
		return apperrors.Validation(err.Error())
	}

	fr, err := h.requestSvc.UpdateNotifyEmails(c.Context(), c.Params("id"), claims.UserID, body.Emails)
	if err != nil {
		return err
	}
	return c.JSON(fileRequestResponse(fr))
}

// ResendRequestInvite POST /api/v1/requests/:id/resend
// Re-sends the request_invite email to all current recipients. Owner only.
func (h *Handler) ResendRequestInvite(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(*middleware.Claims)
	if !ok || claims == nil {
		return apperrors.Unauthorized("unauthenticated")
	}

	if err := h.requestSvc.ResendInvite(c.Context(), c.Params("id"), claims.UserID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "notification queued"})
}

// --- Public endpoints (no auth required) ---

// GetPublicFileRequest GET /api/v1/r/:slug
func (h *Handler) GetPublicFileRequest(c fiber.Ctx) error {
	slug := c.Params("slug")
	req, err := h.requestSvc.GetPublic(c.Context(), slug)
	if err != nil {
		return err
	}
	return c.JSON(fileRequestPublicResponse(req))
}

// UploadToRequest POST /api/v1/r/:slug/upload
// Public, rate-limited. Accepts multipart/form-data with a "file" field,
// optional "submitter_name" and "message" fields.
func (h *Handler) UploadToRequest(c fiber.Ctx) error {
	slug := c.Params("slug")

	fh, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "file is required")
	}

	file, err := fh.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to open file")
	}
	defer file.Close() //nolint:errcheck

	// Fetch the request to obtain the owner's UUID for the service token.
	// The storage service requires a valid UUID as owner_id when creating file records.
	req, err := h.requestSvc.GetPublic(c.Context(), slug)
	if err != nil {
		return err
	}

	// Issue a short-lived service JWT using the request owner's UUID as the subject,
	// so the storage service can store the file record with a valid owner_id.
	token, err := h.issueServiceToken(req.OwnerID)
	if err != nil {
		return apperrors.Internal("issue service token", err)
	}

	sub, err := h.requestSvc.Submit(c.Context(), service.SubmitParams{
		Slug:          slug,
		File:          file,
		Header:        fh,
		SubmitterName: c.FormValue("submitter_name"),
		Message:       c.FormValue("message"),
		IP:            realClientIP(c),
		ServiceToken:  token,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(submissionResponse(sub))
}

// --- Response helpers ---

func fileRequestResponse(r *domain.FileRequest) fiber.Map {
	recipientEmails := []string{}
	if r.NotifyEmails != "" {
		for _, e := range strings.Split(r.NotifyEmails, ",") {
			if t := strings.TrimSpace(e); t != "" {
				recipientEmails = append(recipientEmails, t)
			}
		}
	}
	return fiber.Map{
		"id":               r.ID,
		"slug":             r.Slug,
		"name":             r.Name,
		"description":      r.Description,
		"allowed_types":    r.AllowedTypes,
		"max_size_mb":      r.MaxSizeMB,
		"max_files":        r.MaxFiles,
		"recipient_emails": recipientEmails,
		"notify_on_upload": r.NotifyOnUpload,
		"expires_at":       r.ExpiresAt.Format(time.RFC3339),
		"is_active":        r.IsActive,
		"is_expired":       r.IsExpired(),
		"created_at":       r.CreatedAt.Format(time.RFC3339),
		"submission_count": r.SubmissionCount,
	}
}

func fileRequestPublicResponse(r *domain.FileRequest) fiber.Map {
	return fiber.Map{
		"slug":          r.Slug,
		"name":          r.Name,
		"description":   r.Description,
		"allowed_types": r.AllowedTypes,
		"max_size_mb":   r.MaxSizeMB,
		"max_files":     r.MaxFiles,
		"expires_at":    r.ExpiresAt.Format(time.RFC3339),
		"is_active":     r.IsActive,
		"is_expired":    r.IsExpired(),
	}
}

func submissionResponse(s *domain.RequestSubmission) fiber.Map {
	return fiber.Map{
		"id":             s.ID,
		"file_id":        s.FileID,
		"filename":       s.Filename,
		"size_bytes":     s.SizeBytes,
		"submitter_name": s.SubmitterName,
		"message":        s.Message,
		"submitted_at":   s.SubmittedAt.Format(time.RFC3339),
	}
}
