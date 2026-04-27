package handlers

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/storage/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/storage/internal/repository"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
	sharedStorage "github.com/tenzoshare/tenzoshare/shared/pkg/storage"
)

type Handler struct {
	repo          *repository.FileRepository
	backend       sharedStorage.Backend
	js            *jetstream.Client
	log           *zap.Logger
	encryptionKey []byte // 32-byte AES-256 master key; nil = encryption disabled
}

func New(repo *repository.FileRepository, backend sharedStorage.Backend, js *jetstream.Client, log *zap.Logger, encryptionKeyHex string) *Handler {
	var encKey []byte
	if encryptionKeyHex != "" {
		k, err := hex.DecodeString(encryptionKeyHex)
		if err != nil || len(k) != 32 {
			log.Fatal("STORAGE_ENCRYPTION_KEY must be a 64-char hex string (32 bytes)")
		}
		encKey = k
	} else {
		log.Warn("STORAGE_ENCRYPTION_KEY not set — files will be stored unencrypted")
	}
	return &Handler{repo: repo, backend: backend, js: js, log: log, encryptionKey: encKey}
}

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
	defer f.Close() //nolint:errcheck

	plaintext, err := io.ReadAll(f)
	if err != nil {
		return apperrors.Internal("read uploaded file", err)
	}

	objectKey := fmt.Sprintf("uploads/%s/%s/%s", ownerID, uuid.New().String(), fileHeader.Filename)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var uploadData []byte
	var encIV []byte

	if h.encryptionKey != nil {
		ciphertext, iv, encErr := encryptAESGCM(plaintext, h.encryptionKey)
		if encErr != nil {
			return apperrors.Internal("encrypt file", encErr)
		}
		uploadData = ciphertext
		encIV = iv
	} else {
		uploadData = plaintext
	}

	if err := h.backend.Upload(c.Context(), objectKey, bytes.NewReader(uploadData), int64(len(uploadData)), contentType); err != nil {
		return apperrors.Internal("upload to object store", err)
	}

	record, err := h.repo.Create(c.Context(), ownerID, objectKey, fileHeader.Filename, contentType, int64(len(plaintext)), encIV)
	if err != nil {
		_ = h.backend.Delete(c.Context(), objectKey) // best-effort cleanup
		return err
	}

	h.publishAudit(c.Context(), "storage.upload", ownerID, record.ID, map[string]any{
		"filename": record.Filename,
		"size":     record.SizeBytes,
	})
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
	go func() { _ = h.backend.Delete(context.Background(), file.ObjectKey) }() //nolint:errcheck

	h.publishAudit(c.Context(), "storage.delete", ownerID, id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// downloadClaims is the payload for short-lived file download tokens.
type downloadClaims struct {
	FileID string `json:"fid"`
	jwt.RegisteredClaims
}

// issueDownloadToken mints a 15-minute HS256 JWT authorising download of fileID.
// The encryptionKey (32 bytes) is reused as the HMAC secret.
func (h *Handler) issueDownloadToken(fileID string) (string, error) {
	claims := downloadClaims{
		FileID: fileID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "dl",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(h.encryptionKey)
}

// PresignURL returns a short-lived download URL.
// For encrypted files it embeds a download token so the browser can navigate
// to the URL directly; for unencrypted files it returns a MinIO presigned URL.
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

	// Encrypted files: embed a short-lived download token in the URL so the
	// browser can navigate without needing a separate Authorization header.
	if file.IsEncrypted {
		if h.encryptionKey != nil {
			dlToken, err := h.issueDownloadToken(id)
			if err != nil {
				return apperrors.Internal("issue download token", err)
			}
			return c.JSON(fiber.Map{
				"url":        fmt.Sprintf("/api/v1/files/%s/download?token=%s", id, dlToken),
				"expires_in": 900,
				"encrypted":  true,
			})
		}
		// Fallback (shouldn't occur — encrypted files require an encryption key)
		return c.JSON(fiber.Map{
			"url":        nil,
			"download":   fmt.Sprintf("/api/v1/files/%s/download", id),
			"expires_in": 900,
			"encrypted":  true,
		})
	}

	url, err := h.backend.GetPresignedURL(c.Context(), file.ObjectKey, file.Filename, 15*time.Minute)
	if err != nil {
		return apperrors.Internal("generate presigned url", err)
	}

	return c.JSON(fiber.Map{"url": url, "expires_in": 900, "encrypted": false})
}

// Download streams a (possibly encrypted) file.
// GET /api/v1/files/:id/download
// Accepts either:
//   - Bearer JWT (RS256) via Authorization header — authenticated user/service
//   - ?token=<jwt> query param — HS256 download token issued by PresignURL
func (h *Handler) Download(c fiber.Ctx) error {
	id := c.Params("id")
	ownerID, _ := c.Locals("userID").(string)
	role, _ := c.Locals("userRole").(string)

	// If no Bearer JWT was validated by OptionalJWTAuth, check the ?token= param.
	if ownerID == "" {
		dlToken := c.Query("token")
		if dlToken == "" {
			return apperrors.Unauthorized("authentication required")
		}
		if h.encryptionKey == nil {
			return apperrors.Unauthorized("download tokens not configured")
		}
		claims := &downloadClaims{}
		tok, err := jwt.ParseWithClaims(dlToken, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return h.encryptionKey, nil
		})
		if err != nil || !tok.Valid || claims.FileID != id {
			return apperrors.Unauthorized("invalid or expired download token")
		}
		// Token is valid for this file — allow download without ownership check.
		role = "admin"
	}

	file, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return err
	}

	if role != "admin" && file.OwnerID != ownerID {
		return apperrors.Forbidden("access denied")
	}

	rc, err := h.backend.Download(c.Context(), file.ObjectKey)
	if err != nil {
		return apperrors.Internal("download from object store", err)
	}
	defer rc.Close() //nolint:errcheck

	raw, err := io.ReadAll(rc)
	if err != nil {
		return apperrors.Internal("read object data", err)
	}

	var plaintext []byte
	if file.IsEncrypted && len(file.EncryptionIV) > 0 && h.encryptionKey != nil {
		plaintext, err = decryptAESGCM(raw, h.encryptionKey, file.EncryptionIV)
		if err != nil {
			return apperrors.Internal("decrypt file", err)
		}
	} else {
		plaintext = raw
	}

	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, file.Filename))
	c.Set("Content-Type", file.ContentType)
	return c.Send(plaintext)
}

func fileResponse(f *domain.File) fiber.Map {
	return fiber.Map{
		"id":           f.ID,
		"owner_id":     f.OwnerID,
		"filename":     f.Filename,
		"content_type": f.ContentType,
		"size_bytes":   f.SizeBytes,
		"is_encrypted": f.IsEncrypted,
		"created_at":   f.CreatedAt,
	}
}

// GetMyUsage returns storage consumption for the authenticated user.
// GET /api/v1/files/usage
func (h *Handler) GetMyUsage(c fiber.Ctx) error {
	ownerID, _ := c.Locals("userID").(string)
	if ownerID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	usage, err := h.repo.GetUsageByOwner(c.Context(), ownerID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"user_id":     usage.UserID,
		"file_count":  usage.FileCount,
		"total_bytes": usage.TotalBytes,
	})
}

func (h *Handler) publishAudit(ctx context.Context, action, userID, fileID string, extra map[string]any) {
	if h.js == nil {
		return
	}
	ev := map[string]any{
		"action":    action,
		"user_id":   userID,
		"file_id":   fileID,
		"success":   true,
		"timestamp": time.Now(),
	}
	for k, v := range extra {
		ev[k] = v
	}
	go func() {
		if err := h.js.Publish(ctx, "AUDIT.storage", ev); err != nil {
			h.log.Warn("failed to publish storage audit event", zap.String("action", action), zap.Error(err))
		}
	}()
}

// encryptAESGCM encrypts plaintext with AES-256-GCM using the provided 32-byte key.
// Returns (ciphertext, iv, error). The IV (nonce) is stored in the DB and prepended to
// the ciphertext in the stored object for self-contained recovery.
func encryptAESGCM(plaintext, key []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	iv := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, nil, err
	}
	// Seal appends ciphertext+tag to iv → [iv | ciphertext | tag]
	ciphertext := gcm.Seal(iv, iv, plaintext, nil)
	return ciphertext, iv, nil
}

// decryptAESGCM decrypts AES-256-GCM ciphertext. The ciphertext must start with the IV.
func decryptAESGCM(ciphertext, key, _ []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	iv, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, iv, ct, nil)
}
