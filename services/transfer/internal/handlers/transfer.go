package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/service"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jwtkeys"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
)

// errFileDeleted is returned by fetchPresignURL when the storage service reports
// that the requested file no longer exists (soft-deleted by admin or retention worker).
var errFileDeleted = errors.New("file no longer available")

// policyCache holds the link_protection_policy fetched from the admin service,
// refreshed at most every 5 minutes.
type policyCache struct {
	mu        sync.Mutex
	value     string
	fetchedAt time.Time
	adminURL  string
}

func (pc *policyCache) get(ctx context.Context) string {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if time.Since(pc.fetchedAt) < 5*time.Minute && pc.value != "" {
		return pc.value
	}
	// Fetch from admin public endpoint.
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, pc.adminURL+"/api/v1/platform/config", nil)
	if err != nil {
		return "none"
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "none" // fail-open: don't block transfers if admin is down
	}
	defer resp.Body.Close()
	var cfg struct {
		LinkProtectionPolicy string `json:"link_protection_policy"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return "none"
	}
	if cfg.LinkProtectionPolicy == "" {
		cfg.LinkProtectionPolicy = "none"
	}
	pc.value = cfg.LinkProtectionPolicy
	pc.fetchedAt = time.Now()
	return pc.value
}

type Handler struct {
	svc           *service.TransferService
	requestSvc    *service.RequestService
	validate      *validator.Validate
	jwtPrivateKey *rsa.PrivateKey
	storageURL    string
	policy        *policyCache
}

func New(svc *service.TransferService, requestSvc *service.RequestService, jwtPrivateKeyPEM, storageURL, adminURL string) *Handler {
	privKey, err := jwtkeys.ParsePrivateKey(jwtPrivateKeyPEM)
	if err != nil {
		// panic at startup — private key is required for service-to-service tokens
		panic("transfer handler: " + err.Error())
	}
	return &Handler{
		svc:           svc,
		requestSvc:    requestSvc,
		validate:      validator.New(),
		jwtPrivateKey: privKey,
		storageURL:    storageURL,
		policy:        &policyCache{adminURL: adminURL},
	}
}

type createRequest struct {
	Name        string   `json:"name"             validate:"required,min=1,max=200"`
	Description string   `json:"description"      validate:"max=1000"`
	FileIDs     []string `json:"file_ids"         validate:"required,min=1,dive,uuid4"`
	// RecipientEmails accepts multiple addresses; stored comma-separated.
	// RecipientEmail is kept for backward-compat single-email clients.
	RecipientEmails  []string `json:"recipient_emails" validate:"omitempty,max=20,dive,email"`
	RecipientEmail   string   `json:"recipient_email"  validate:"omitempty,email"`
	Password         string   `json:"password"`
	MaxDownloads     int      `json:"max_downloads"    validate:"min=0"`
	ViewOnly         bool     `json:"view_only"`
	NotifyOnDownload bool     `json:"notify_on_download"`
	ExpiresInHours   int      `json:"expires_in_hours" validate:"required,min=1,max=2160"`
}

// Create POST /api/v1/transfers
func (h *Handler) Create(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	claims, _ := c.Locals("claims").(*middleware.Claims)
	senderEmail := ""
	if claims != nil {
		senderEmail = claims.Email
	}

	var req createRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid JSON")
	}
	if err := h.validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	// Merge both fields: prefer the array, fall back to the single-address field.
	allEmails := req.RecipientEmails
	if len(allEmails) == 0 && req.RecipientEmail != "" {
		allEmails = []string{req.RecipientEmail}
	}
	recipientEmail := strings.Join(allEmails, ",")

	// Enforce admin link-protection policy.
	switch h.policy.get(c.Context()) {
	case "password":
		if req.Password == "" {
			return apperrors.Validation("this platform requires all transfers to be protected by a password")
		}
	case "email":
		if len(allEmails) == 0 {
			return apperrors.Validation("this platform requires all transfers to have at least one recipient email")
		}
	case "either":
		if req.Password == "" && len(allEmails) == 0 {
			return apperrors.Validation("this platform requires all transfers to have a password or at least one recipient email")
		}
	}

	result, err := h.svc.Create(c.Context(), service.CreateParams{
		OwnerID:          ownerID,
		SenderEmail:      senderEmail,
		Name:             req.Name,
		Description:      req.Description,
		FileIDs:          req.FileIDs,
		RecipientEmail:   recipientEmail,
		Password:         req.Password,
		MaxDownloads:     req.MaxDownloads,
		ViewOnly:         req.ViewOnly,
		NotifyOnDownload: req.NotifyOnDownload,
		ExpiresIn:        time.Duration(req.ExpiresInHours) * time.Hour,
		ClientIP:         realClientIP(c),
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

	// By default exclude expired and revoked; exhausted transfers are included.
	// Pass ?status=all to include everything.
	statusFilter := c.Query("status")
	items := make([]fiber.Map, 0, len(transfers))
	for _, t := range transfers {
		s := t.Status()
		if statusFilter != "all" && s != "active" && s != "exhausted" {
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
// GET /api/v1/t/:slug  — optionally with ?password= or ?rt= (recipient token)
// Does NOT increment any counter — viewing the download page is not a download.
// Returns per-file download counts (file_download_counts map) when max_downloads > 0
// so the download UI can show per-file availability without requiring an attempt.
func (h *Handler) Access(c fiber.Ctx) error {
	slug := c.Params("slug")

	var result *service.AccessResult
	var err error

	if rt := c.Query("rt"); rt != "" {
		// Recipient magic-link token path — bypasses password requirement.
		result, err = h.svc.ValidateRecipientToken(c.Context(), slug, rt)
	} else {
		result, err = h.svc.Validate(c.Context(), service.AccessParams{
			Slug:     slug,
			Password: c.Query("password"),
		})
	}
	if err != nil {
		return err
	}

	resp := transferResponse(result.Transfer, result.FileIDs)
	if result.FileDownloadCounts != nil {
		resp["file_download_counts"] = result.FileDownloadCounts
	}
	if result.FileInfos != nil {
		type fileInfoJSON struct {
			ID           string `json:"id"`
			Filename     string `json:"filename"`
			ContentType  string `json:"content_type"`
			SizeBytes    int64  `json:"size_bytes"`
			DeleteReason string `json:"delete_reason"` // empty = available; see repository.FileInfo
		}
		infos := make([]fileInfoJSON, len(result.FileInfos))
		for i, f := range result.FileInfos {
			infos[i] = fileInfoJSON{ID: f.ID, Filename: f.Filename, ContentType: f.ContentType, SizeBytes: f.SizeBytes, DeleteReason: f.DeleteReason}
		}
		resp["files"] = infos
	}

	return c.JSON(fiber.Map{
		"transfer": resp,
	})
}

// RequestAccess handles POST /api/v1/t/:slug/request-access.
// A recipient whose magic-link has expired can submit their email to get a new link.
// We never reveal whether the email is actually a recipient (prevents enumeration).
func (h *Handler) RequestAccess(c fiber.Ctx) error {
	slug := c.Params("slug")
	var body struct {
		Email string `json:"email"`
	}
	if err := c.Bind().JSON(&body); err != nil || body.Email == "" {
		return apperrors.BadRequest("email is required")
	}
	// Normalise + basic validation.
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if !strings.Contains(body.Email, "@") {
		return apperrors.BadRequest("invalid email address")
	}

	// Fire-and-forget: always return 200 regardless of outcome (no oracle).
	go func() {
		ctx := context.Background()
		if err := h.svc.RegenerateRecipientToken(ctx, slug, body.Email); err != nil {
			// Log only — do not surface error to caller.
			_ = err
		}
	}()

	return c.JSON(fiber.Map{"message": "If that email is a recipient of this transfer, a new access link has been sent."})
}

func transferResponse(t *domain.Transfer, fileIDs []string) fiber.Map {
	m := fiber.Map{
		"id":                 t.ID,
		"owner_id":           t.OwnerID,
		"sender_email":       t.SenderEmail,
		"name":               t.Name,
		"description":        t.Description,
		"slug":               t.Slug,
		"status":             t.Status(),
		"max_downloads":      t.MaxDownloads,
		"download_count":     t.DownloadCount,
		"is_revoked":         t.IsRevoked,
		"view_only":          t.ViewOnly,
		"notify_on_download": t.NotifyOnDownload,
		"has_password":       t.PasswordHash != "",
		"created_at":         t.CreatedAt,
		"file_count":         t.FileCount,
		"total_size_bytes":   t.TotalSizeBytes,
	}
	if t.RecipientEmail != "" {
		m["recipient_email"] = t.RecipientEmail
		// Also expose as an array for multi-recipient-aware clients.
		parts := strings.Split(t.RecipientEmail, ",")
		emails := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				emails = append(emails, s)
			}
		}
		m["recipient_emails"] = emails
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
// GET /api/v1/t/:slug/files/:fileId/download[?password=...][?rt=...]
//
// This endpoint:
//  1. Validates the transfer (slug, optional password or recipient token, expiry, revocation).
//  2. Confirms the requested file belongs to this transfer.
//  3. Atomically checks and increments the per-file download counter. If the
//     file's individual limit is reached the request is rejected with 403.
//     This prevents downloading file A from consuming quota for file B.
//  4. Issues a short-lived internal service JWT and proxies to the Storage service
//     presign endpoint. No auth is required from the caller.
//
// Response: { "url": "<presigned MinIO URL>", "expires_in": 900 }
func (h *Handler) DownloadURL(c fiber.Ctx) error {
	slug := c.Params("slug")
	fileID := c.Params("fileId")

	// If a recipient token is provided, validate it first to bypass the password.
	// We still call AttemptFileDownload for limit enforcement; pass empty password
	// when the token was valid (transfer has already been verified above).
	var password string
	if rt := c.Query("rt"); rt != "" {
		if _, err := h.svc.ValidateRecipientToken(c.Context(), slug, rt); err != nil {
			return err
		}
		// Token valid — password not needed.
	} else {
		password = c.Query("password")
	}

	// AttemptFileDownload validates the transfer, confirms file ownership, and
	// atomically enforces the per-file download limit.
	result, err := h.svc.AttemptFileDownload(c.Context(), service.AttemptFileDownloadParams{
		Slug:     slug,
		FileID:   fileID,
		Password: password,
		ClientIP: realClientIP(c),
	})
	if err != nil {
		return err
	}

	// Issue a short-lived (30 s) service JWT so the Storage service accepts the request.
	svcToken, err := h.issueServiceToken(result.Transfer.OwnerID)
	if err != nil {
		return apperrors.Internal("issue service token", err)
	}

	// Proxy the presign request to the Storage service.
	presignURL, err := h.fetchPresignURL(c.Context(), fileID, svcToken)
	if err != nil {
		if errors.Is(err, errFileDeleted) {
			return apperrors.NotFound("this file has been deleted and is no longer available for download")
		}
		return apperrors.Internal("get presigned url from storage service", err)
	}

	// For view-only transfers, ensure the file is served with Content-Disposition: inline
	// so the browser renders it rather than prompting a save dialog.
	// This only applies to encrypted files served via our own proxy endpoint
	// (URLs beginning with /api/v1/files/); raw MinIO presigned URLs cannot have
	// headers injected and are handled at the UI level instead.
	viewOnly := result.Transfer.ViewOnly
	if viewOnly && strings.HasPrefix(presignURL, "/api/v1/files/") {
		if strings.Contains(presignURL, "?") {
			presignURL += "&inline=1"
		} else {
			presignURL += "?inline=1"
		}
	}

	return c.JSON(fiber.Map{"url": presignURL, "expires_in": 900, "view_only": viewOnly})
}

// UpdateRecipients PATCH /api/v1/transfers/:id/recipients
// Replaces the recipient list for a transfer. Owner only.
func (h *Handler) UpdateRecipients(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	transferID := c.Params("id")

	var body struct {
		Emails []string `json:"emails" validate:"omitempty,max=20,dive,email"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return apperrors.BadRequest("invalid JSON")
	}
	if err := h.validate.Struct(body); err != nil {
		return apperrors.Validation(err.Error())
	}

	t, err := h.svc.UpdateRecipients(c.Context(), transferID, ownerID, body.Emails)
	if err != nil {
		return err
	}
	return c.JSON(transferResponse(t, nil))
}

// ResendNotification POST /api/v1/transfers/:id/resend
// Re-sends the transfer_received email to all current recipients. Owner only.
func (h *Handler) ResendNotification(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	transferID := c.Params("id")

	if err := h.svc.ResendNotification(c.Context(), transferID, ownerID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "notification queued"})
}

// ListRecipients GET /api/v1/transfers/:id/recipients
// Returns the recipient(s) for a transfer.
// Only the transfer owner may call this endpoint.
func (h *Handler) ListRecipients(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	transferID := c.Params("id")
	if transferID == "" {
		return apperrors.Validation("transfer id required")
	}

	t, err := h.svc.GetByID(c.Context(), transferID)
	if err != nil {
		return err
	}
	if t.OwnerID != ownerID {
		return apperrors.Forbidden("access denied")
	}

	type recipient struct {
		Email         string `json:"email"`
		DownloadCount int    `json:"download_count"`
		MaxDownloads  int    `json:"max_downloads"`
	}

	recipients := []recipient{}
	if t.RecipientEmail != "" {
		recipients = append(recipients, recipient{
			Email:         t.RecipientEmail,
			DownloadCount: t.DownloadCount,
			MaxDownloads:  t.MaxDownloads,
		})
	}

	return c.JSON(fiber.Map{"recipients": recipients})
}

// issueServiceToken mints a short-lived (30 s) RS256 JWT with role=admin for
// internal service-to-service calls. The Storage service's JWT middleware accepts it.
// subject must be a valid UUID (used as owner_id by the Storage service).
func (h *Handler) issueServiceToken(subject string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  subject,
		"role": "admin",
		"jti":  uuid.New().String(),
		"iat":  now.Unix(),
		"exp":  now.Add(30 * time.Second).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return t.SignedString(h.jwtPrivateKey)
}

// fetchDownloadURL calls the Storage service's presign endpoint.
// If the file is encrypted, the storage service returns a {download: "/api/v1/files/:id/download"}
// marker and the transfer service builds a proxy URL instead.
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
	if resp.StatusCode == http.StatusNotFound {
		// File was deleted (storage service returns 404 for soft-deleted files).
		return "", errFileDeleted
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("storage service returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response — may contain {url, encrypted} or {download, encrypted}
	var parsed struct {
		URL       *string `json:"url"`
		Download  string  `json:"download"`
		Encrypted bool    `json:"encrypted"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse storage response: %w", err)
	}

	// Encrypted files: storage now returns a path with an embedded download token.
	// Return it as-is so the client resolves it relative to the gateway (browser-navigable).
	if parsed.Encrypted {
		if parsed.URL != nil && *parsed.URL != "" {
			return *parsed.URL, nil
		}
		// Legacy fallback: storage returned download path only (before token support)
		if parsed.Download != "" {
			return fmt.Sprintf("%s%s", h.storageURL, parsed.Download), nil
		}
		return "", fmt.Errorf("storage returned encrypted flag but no download URL")
	}
	if parsed.URL == nil {
		return "", fmt.Errorf("storage returned no download URL")
	}
	return *parsed.URL, nil
}
