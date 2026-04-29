package handlers

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
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

	// ── Enforce storage policy (quota + max upload size) ─────────────────────
	cfg, cfgErr := h.repo.GetStorageConfig(c.Context())
	if cfgErr == nil {
		// In production mode (test_mode disabled), reject uploads that arrive
		// over plain HTTP. TLS may be terminated at the reverse proxy, so we
		// check X-Forwarded-Proto in addition to the raw connection state.
		if !cfg.TestMode {
			isHTTPS := c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https")
			if !isHTTPS {
				h.log.Warn("upload rejected: plain HTTP connection in production mode",
					zap.String("owner_id", ownerID),
					zap.String("filename", fileHeader.Filename),
				)
				h.publishAuditFail(c.Context(), "storage.upload_rejected", ownerID, "http_not_allowed",
					map[string]any{"filename": fileHeader.Filename, "size": fileHeader.Size})
				return fiber.NewError(fiber.StatusForbidden,
					"uploads require a secure HTTPS connection — plain HTTP is not permitted. "+
						"If you are a system administrator, enable Test Mode in Storage Settings to allow HTTP uploads.")
			}
		}

		// Per-file size cap
		if cfg.MaxUploadSizeBytes > 0 && fileHeader.Size > cfg.MaxUploadSizeBytes {
			msg := fmt.Sprintf("file exceeds maximum upload size of %s", fmtBytes(cfg.MaxUploadSizeBytes))
			h.log.Warn("upload rejected: file too large",
				zap.String("owner_id", ownerID),
				zap.String("filename", fileHeader.Filename),
				zap.Int64("size", fileHeader.Size),
				zap.Int64("limit", cfg.MaxUploadSizeBytes),
			)
			h.publishAuditFail(c.Context(), "storage.upload_rejected", ownerID, "file_too_large",
				map[string]any{"filename": fileHeader.Filename, "size": fileHeader.Size, "limit": cfg.MaxUploadSizeBytes})
			return apperrors.BadRequest(msg)
		}
		// Per-user quota
		if cfg.QuotaEnabled && cfg.QuotaBytesPerUser > 0 {
			usage, uErr := h.repo.GetUsageByOwner(c.Context(), ownerID)
			if uErr == nil && usage.TotalBytes+fileHeader.Size > cfg.QuotaBytesPerUser {
				msg := fmt.Sprintf("storage quota exceeded: %s used of %s",
					fmtBytes(usage.TotalBytes),
					fmtBytes(cfg.QuotaBytesPerUser),
				)
				h.log.Warn("upload rejected: quota exceeded",
					zap.String("owner_id", ownerID),
					zap.String("filename", fileHeader.Filename),
					zap.Int64("size", fileHeader.Size),
					zap.Int64("used", usage.TotalBytes),
					zap.Int64("quota", cfg.QuotaBytesPerUser),
				)
				h.publishAuditFail(c.Context(), "storage.upload_rejected", ownerID, "quota_exceeded",
					map[string]any{"filename": fileHeader.Filename, "size": fileHeader.Size, "used": usage.TotalBytes, "quota": cfg.QuotaBytesPerUser})
				return apperrors.BadRequest(msg)
			}
		}
	}
	// ─────────────────────────────────────────────────────────────────────────

	f, err := fileHeader.Open()
	if err != nil {
		return apperrors.Internal("open uploaded file", err)
	}
	defer f.Close() //nolint:errcheck

	// Sanitize the filename to prevent path traversal and strip control characters.
	filename := sanitizeFilename(fileHeader.Filename)
	objectKey := fmt.Sprintf("uploads/%s/%s/%s", ownerID, uuid.New().String(), filename)

	// Sniff the real content type from the first 512 bytes.
	// Browsers frequently send "application/octet-stream" for types they don't recognise,
	// which would cause incorrect icons and broken inline previews on download.
	sniffBuf := make([]byte, 512)
	sniffN, _ := io.ReadFull(f, sniffBuf)
	sniffBuf = sniffBuf[:sniffN]
	browserType := fileHeader.Header.Get("Content-Type")
	contentType := browserType
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(sniffBuf)
	}
	// Reconstruct the full stream: re-prepend the already-read bytes.
	fullReader := io.MultiReader(bytes.NewReader(sniffBuf), f)

	// Stream to MinIO without loading the file fully into memory.
	// Encrypted uploads use chunked streaming AES-256-GCM.
	var uploadReader io.Reader
	var uploadSize int64
	var encIV []byte

	if h.encryptionKey != nil && fileHeader.Size > 0 {
		baseIV := make([]byte, 12)
		if _, err := io.ReadFull(rand.Reader, baseIV); err != nil {
			return apperrors.Internal("generate encryption IV", err)
		}
		enc, encErr := newStreamEncryptor(fullReader, h.encryptionKey, baseIV)
		if encErr != nil {
			return apperrors.Internal("init encryption", encErr)
		}
		uploadReader = enc
		uploadSize = chunkedCiphertextSize(fileHeader.Size)
		encIV = makeChunkedIV(baseIV)
	} else {
		// Unencrypted (or zero-length): stream directly to MinIO.
		uploadReader = fullReader
		uploadSize = fileHeader.Size
	}

	if err := h.backend.Upload(c.Context(), objectKey, uploadReader, uploadSize, contentType); err != nil {
		h.log.Error("upload to object store failed",
			zap.String("owner_id", ownerID),
			zap.String("filename", filename),
			zap.Int64("size", fileHeader.Size),
			zap.Error(err),
		)
		h.publishAuditFail(c.Context(), "storage.upload_failed", ownerID, "s3_error",
			map[string]any{"filename": filename, "size": fileHeader.Size})
		return apperrors.Internal("upload to object store", err)
	}

	record, err := h.repo.Create(c.Context(), ownerID, objectKey, filename, contentType, fileHeader.Size, encIV)
	if err != nil {
		_ = h.backend.Delete(c.Context(), objectKey) // best-effort cleanup
		h.log.Error("failed to create file record",
			zap.String("owner_id", ownerID),
			zap.String("filename", filename),
			zap.Error(err),
		)
		h.publishAuditFail(c.Context(), "storage.upload_failed", ownerID, "db_error",
			map[string]any{"filename": filename, "size": fileHeader.Size})
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

// ListFiles returns all files owned by the authenticated user, including share
// counts and the computed auto-delete date (when retention is enabled).
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

	files, err := h.repo.ListByOwnerWithShareInfo(c.Context(), ownerID, limit, offset)
	if err != nil {
		return err
	}

	// Load retention config so we can compute auto_delete_at for each file.
	cfg, _ := h.repo.GetStorageConfig(c.Context())

	items := make([]fiber.Map, 0, len(files))
	for _, f := range files {
		m := fileResponse(&f.File)
		m["share_count"] = f.ShareCount
		m["active_shares"] = f.ActiveShares
		if f.LastShareExpiresAt != nil {
			m["last_share_expires_at"] = f.LastShareExpiresAt.Format(time.RFC3339)
		} else {
			m["last_share_expires_at"] = nil
		}
		// Compute auto_delete_at:
		// - nil if retention is disabled
		// - nil if there's at least one active never-expiring share
		// - created_at + orphan_retention_days for files that were never shared
		// - last_share_expires_at + retention_days for files whose shares all ended
		if cfg != nil && cfg.RetentionEnabled {
			var autoDelete *time.Time
			if f.ShareCount == 0 {
				// Orphan: will be deleted after orphan_retention_days from upload
				t := f.CreatedAt.Add(time.Duration(cfg.OrphanRetentionDays) * 24 * time.Hour)
				autoDelete = &t
			} else if f.ActiveShares == 0 && f.LastShareExpiresAt != nil {
				// All shares expired: will be deleted after retention_days from last expiry
				t := f.LastShareExpiresAt.Add(time.Duration(cfg.RetentionDays) * 24 * time.Hour)
				autoDelete = &t
			}
			// else: active shares exist → file is protected → auto_delete_at stays nil
			if autoDelete != nil {
				m["auto_delete_at"] = autoDelete.Format(time.RFC3339)
			} else {
				m["auto_delete_at"] = nil
			}
		} else {
			m["auto_delete_at"] = nil
		}
		items = append(items, m)
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

	disposition := "attachment"
	if c.Query("inline") == "1" {
		disposition = "inline"
	}
	c.Set("Content-Disposition", fmt.Sprintf(`%s; filename=%q`, disposition, file.Filename))
	c.Set("Content-Type", file.ContentType)

	if file.IsEncrypted && len(file.EncryptionIV) > 0 && h.encryptionKey != nil {
		if isChunkedEncryption(file.EncryptionIV) {
			// New streaming chunked format — decrypt on the fly, no full-file buffering.
			dec, decErr := newStreamDecryptor(rc, h.encryptionKey)
			if decErr != nil {
				return apperrors.Internal("init decryption", decErr)
			}
			return c.SendStream(dec, int(file.SizeBytes))
		}
		// Legacy single-GCM block (files created before this fix; always < 4 MiB).
		raw, err := io.ReadAll(rc)
		if err != nil {
			return apperrors.Internal("read object data", err)
		}
		plain, err := decryptAESGCM(raw, h.encryptionKey, file.EncryptionIV)
		if err != nil {
			return apperrors.Internal("decrypt file", err)
		}
		return c.Send(plain)
	}

	// Unencrypted — stream directly from object store without buffering.
	return c.SendStream(rc, int(file.SizeBytes))
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

// GetMyUsage returns storage consumption for the authenticated user,
// including the applicable quota settings so the client can render a progress bar.
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

	// Include quota and upload-limit settings so the UI can show a progress bar
	// and pre-validate file sizes client-side before sending them to the server.
	cfg, cfgErr := h.repo.GetStorageConfig(c.Context())
	quotaEnabled := false
	var quotaBytes, maxUploadBytes int64
	if cfgErr == nil {
		quotaEnabled = cfg.QuotaEnabled
		quotaBytes = cfg.QuotaBytesPerUser
		maxUploadBytes = cfg.MaxUploadSizeBytes
	}

	return c.JSON(fiber.Map{
		"user_id":               usage.UserID,
		"file_count":            usage.FileCount,
		"total_bytes":           usage.TotalBytes,
		"quota_enabled":         quotaEnabled,
		"quota_bytes_per_user":  quotaBytes,
		"max_upload_size_bytes": maxUploadBytes,
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

// publishAuditFail publishes a failure audit event for storage operations.
func (h *Handler) publishAuditFail(ctx context.Context, action, userID, reason string, extra map[string]any) {
	if h.js == nil {
		return
	}
	ev := map[string]any{
		"action":    action,
		"user_id":   userID,
		"success":   false,
		"timestamp": time.Now(),
		"reason":    reason,
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

// sanitizeFilename strips path-traversal components and unsafe characters from a
// user-supplied filename so it is safe to use as an S3 object-key suffix and in
// HTTP Content-Disposition headers.
func sanitizeFilename(raw string) string {
	// Normalise to forward slashes then take only the final component,
	// eliminating "../", absolute paths, and Windows backslashes.
	name := path.Base(filepath.ToSlash(raw))
	// Strip ASCII control characters (null bytes, carriage returns, etc.).
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, name)
	// Remove trailing dots and spaces (unsafe on some filesystems).
	name = strings.TrimRight(name, ". ")
	name = strings.TrimSpace(name)
	// Enforce 255-byte maximum (common filesystem and S3 key-component limit).
	for len(name) > 255 {
		ext := filepath.Ext(name)
		stem := name[:len(name)-len(ext)]
		cutTo := max(1, 255-len(ext))
		if len(stem) <= cutTo {
			break
		}
		name = stem[:cutTo] + ext
	}
	if name == "" || name == "." || name == ".." {
		name = "upload"
	}
	return name
}

// fmtBytes formats a byte count as a human-readable string (B/KB/MB/GB).
func fmtBytes(b int64) string {
	const k = 1024
	switch {
	case b < k:
		return fmt.Sprintf("%d B", b)
	case b < k*k:
		return fmt.Sprintf("%.1f KB", float64(b)/k)
	case b < k*k*k:
		return fmt.Sprintf("%.1f MB", float64(b)/(k*k))
	default:
		return fmt.Sprintf("%.2f GB", float64(b)/(k*k*k))
	}
}

// ── Chunked streaming AES-256-GCM encryption ─────────────────────────────────
//
// Format stored in MinIO for each encrypted object:
//   [4-byte BE plaintext_len][12-byte chunk_IV][ciphertext (plaintext_len + 16 GCM tag)] ...
//
// The DB stores a 20-byte EncryptionIV: [8-byte magic "TZCHUNK\x01"][12-byte base_IV].
// Each chunk's IV is derived as: base_IV XOR bigEndian(chunk_index, 8 bytes, right-aligned).
// Legacy files have a 12-byte EncryptionIV and use the old single-GCM code path.
// ─────────────────────────────────────────────────────────────────────────────

// uploadChunkSize is the plaintext chunk size for streaming encryption.
// 2 MiB balances RAM usage (≤2 MiB in-flight per upload/download) against
// per-chunk overhead (32 bytes — negligible even for multi-GB files).
const uploadChunkSize = 2 * 1024 * 1024 // 2 MiB

// chunkMagic identifies the new streaming chunked encryption format.
var chunkMagic = [8]byte{'T', 'Z', 'C', 'H', 'U', 'N', 'K', 0x01}

// isChunkedEncryption reports whether iv uses the new 20-byte chunked format.
func isChunkedEncryption(iv []byte) bool {
	if len(iv) != 20 {
		return false
	}
	for i, b := range chunkMagic {
		if iv[i] != b {
			return false
		}
	}
	return true
}

// makeChunkedIV builds the 20-byte IV marker stored in the DB.
func makeChunkedIV(baseIV []byte) []byte {
	out := make([]byte, 20)
	copy(out[:8], chunkMagic[:])
	copy(out[8:], baseIV)
	return out
}

// deriveChunkIV returns a per-chunk IV by XORing the base IV with the chunk index.
func deriveChunkIV(baseIV []byte, idx uint64) []byte {
	iv := make([]byte, 12)
	copy(iv, baseIV)
	for i := 0; i < 8; i++ {
		iv[11-i] ^= byte(idx >> (uint(i) * 8))
	}
	return iv
}

// chunkedCiphertextSize computes the MinIO object size for a chunked-encrypted file.
// Each chunk adds 4 (len header) + 12 (IV) + 16 (GCM tag) = 32 bytes of overhead.
func chunkedCiphertextSize(plainSize int64) int64 {
	numChunks := (plainSize + uploadChunkSize - 1) / uploadChunkSize
	if numChunks == 0 {
		numChunks = 1
	}
	return plainSize + numChunks*32
}

// streamEncryptor wraps an io.Reader and encrypts each chunk on the fly.
// The AES-GCM cipher is initialised once and reused across all chunks,
// avoiding the expensive key-schedule setup on every chunk boundary.
type streamEncryptor struct {
	src      io.Reader
	gcm      cipher.AEAD
	baseIV   []byte
	chunkIdx uint64
	buf      []byte
	eof      bool
}

func newStreamEncryptor(src io.Reader, key, baseIV []byte) (*streamEncryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &streamEncryptor{src: src, gcm: gcm, baseIV: baseIV}, nil
}

func (e *streamEncryptor) fill() error {
	if e.eof {
		return io.EOF
	}
	plain := make([]byte, uploadChunkSize)
	n, err := io.ReadFull(e.src, plain)
	plain = plain[:n]
	switch {
	case n == 0 && (err == io.EOF || err == io.ErrUnexpectedEOF):
		e.eof = true
		return io.EOF
	case err == io.ErrUnexpectedEOF:
		e.eof = true // last partial chunk; fall through to encrypt
	case err != nil:
		return err
	}

	iv := deriveChunkIV(e.baseIV, e.chunkIdx)
	e.chunkIdx++
	ct := e.gcm.Seal(nil, iv, plain, nil) // len(ct) = len(plain) + 16 (GCM auth tag)

	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(plain)))
	e.buf = append(e.buf[:0], hdr[:]...)
	e.buf = append(e.buf, iv...)
	e.buf = append(e.buf, ct...)
	return nil
}

func (e *streamEncryptor) Read(p []byte) (int, error) {
	for len(e.buf) == 0 {
		if e.eof {
			return 0, io.EOF
		}
		if err := e.fill(); err != nil {
			return 0, err
		}
	}
	n := copy(p, e.buf)
	e.buf = e.buf[n:]
	return n, nil
}

// streamDecryptor wraps an io.Reader and decrypts each chunk on the fly.
// The AES-GCM cipher is initialised once and reused across all chunks.
type streamDecryptor struct {
	src io.Reader
	gcm cipher.AEAD
	buf []byte
	eof bool
}

func newStreamDecryptor(src io.Reader, key []byte) (*streamDecryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &streamDecryptor{src: src, gcm: gcm}, nil
}

func (d *streamDecryptor) fill() error {
	if d.eof {
		return io.EOF
	}
	var hdr [4]byte
	if _, err := io.ReadFull(d.src, hdr[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			d.eof = true
			return io.EOF
		}
		return fmt.Errorf("read chunk header: %w", err)
	}
	plainLen := binary.BigEndian.Uint32(hdr[:])

	iv := make([]byte, 12)
	if _, err := io.ReadFull(d.src, iv); err != nil {
		return fmt.Errorf("read chunk IV: %w", err)
	}

	ct := make([]byte, int(plainLen)+16)
	if _, err := io.ReadFull(d.src, ct); err != nil {
		return fmt.Errorf("read chunk ciphertext: %w", err)
	}

	plain, err := d.gcm.Open(nil, iv, ct, nil)
	if err != nil {
		return fmt.Errorf("decrypt chunk: %w", err)
	}

	d.buf = append(d.buf[:0], plain...)
	return nil
}

func (d *streamDecryptor) Read(p []byte) (int, error) {
	for len(d.buf) == 0 {
		if d.eof {
			return 0, io.EOF
		}
		if err := d.fill(); err != nil {
			return 0, err
		}
	}
	n := copy(p, d.buf)
	d.buf = d.buf[n:]
	return n, nil
}
