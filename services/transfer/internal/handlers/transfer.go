package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/service"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

type Handler struct {
	svc        *service.TransferService
	validate   *validator.Validate
	jwtSecret  string
	storageURL string
}

func New(svc *service.TransferService, jwtSecret, storageURL string) *Handler {
	return &Handler{
		svc:        svc,
		validate:   validator.New(),
		jwtSecret:  jwtSecret,
		storageURL: storageURL,
	}
}

type createRequest struct {
	Name           string   `json:"name"            validate:"required,min=1,max=200"`
	Description    string   `json:"description"     validate:"max=1000"`
	FileIDs        []string `json:"file_ids"        validate:"required,min=1,dive,uuid4"`
	RecipientEmail string   `json:"recipient_email" validate:"omitempty,email"`
	Password       string   `json:"password"`
	MaxDownloads   int      `json:"max_downloads"   validate:"min=0"`
	ExpiresInHours int      `json:"expires_in_hours" validate:"required,min=1,max=2160"`
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

	result, err := h.svc.Create(c.Context(), service.CreateParams{
		OwnerID:        ownerID,
		Name:           req.Name,
		Description:    req.Description,
		FileIDs:        req.FileIDs,
		RecipientEmail: req.RecipientEmail,
		Password:       req.Password,
		MaxDownloads:   req.MaxDownloads,
		ExpiresIn:      time.Duration(req.ExpiresInHours) * time.Hour,
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

	// By default exclude expired and revoked; pass ?status=all to include them.
	statusFilter := c.Query("status")
	items := make([]fiber.Map, 0, len(transfers))
	for _, t := range transfers {
		if statusFilter != "all" && t.Status() != "active" {
			continue
		}
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
// Does NOT increment the download counter — viewing a transfer page is not a download.
func (h *Handler) Access(c fiber.Ctx) error {
	slug := c.Params("slug")
	password := c.Query("password")

	result, err := h.svc.Validate(c.Context(), service.AccessParams{
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
		"id":             t.ID,
		"owner_id":       t.OwnerID,
		"name":           t.Name,
		"description":    t.Description,
		"slug":           t.Slug,
		"status":         t.Status(),
		"max_downloads":  t.MaxDownloads,
		"download_count": t.DownloadCount,
		"is_revoked":     t.IsRevoked,
		"has_password":   t.PasswordHash != "",
		"created_at":     t.CreatedAt,
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

// DownloadURL returns a presigned download URL for a single file in a public transfer.
//
// GET /api/v1/t/:slug/files/:fileId/download[?password=...]
//
// This endpoint:
//  1. Validates the transfer (slug, optional password, expiry, revocation, download limit).
//  2. Confirms the requested file belongs to this transfer.
//  3. Issues a short-lived internal service JWT and proxies to the Storage service
//     presign endpoint. No auth is required from the caller.
//  4. Increments the download counter (this is the actual download action).
//
// Response: { "url": "<presigned MinIO URL>", "expires_in": 900 }
func (h *Handler) DownloadURL(c fiber.Ctx) error {
	slug := c.Params("slug")
	fileID := c.Params("fileId")
	password := c.Query("password")

	// Use Access (which increments the download counter) — this is the real download event.
	result, err := h.svc.Access(c.Context(), service.AccessParams{
		Slug:     slug,
		Password: password,
	})
	if err != nil {
		return err
	}

	// Confirm the file belongs to this transfer.
	found := false
	for _, fid := range result.FileIDs {
		if fid == fileID {
			found = true
			break
		}
	}
	if !found {
		return apperrors.NotFound("file not found in this transfer")
	}

	// Issue a short-lived (30 s) service JWT with role=admin so the Storage
	// service's existing presign endpoint accepts the request without needing the
	// file owner's credentials.
	svcToken, err := h.issueServiceToken()
	if err != nil {
		return apperrors.Internal("issue service token", err)
	}

	// Proxy the presign request to the Storage service.
	presignURL, err := h.fetchPresignURL(c.Context(), fileID, svcToken)
	if err != nil {
		return apperrors.Internal("get presigned url from storage service", err)
	}

	return c.JSON(fiber.Map{"url": presignURL, "expires_in": 900})
}

// issueServiceToken mints a short-lived (30 s) HS256 JWT with role=admin for
// internal service-to-service calls. The Storage service's JWT middleware accepts it.
func (h *Handler) issueServiceToken() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  "service-transfer",
		"role": "admin",
		"iat":  now.Unix(),
		"exp":  now.Add(30 * time.Second).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(h.jwtSecret))
}

// fetchPresignURL calls the Storage service's presign endpoint using a service JWT.
func (h *Handler) fetchPresignURL(ctx context.Context, fileID, token string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/v1/files/%s/presign", h.storageURL, fileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage service unreachable: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("storage service returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse {"url":"...","expires_in":900}
	var parsed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.URL == "" {
		return "", fmt.Errorf("parse storage response: %w", err)
	}
	return parsed.URL, nil
}
