package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/jetstream"
)

const defaultSlugBytes = 32 // 256-bit slug → 43 URL-safe base64 chars — astronomically hard to guess

// transferRepository is the data-access interface required by TransferService.
// It is satisfied by *repository.TransferRepository.
type transferRepository interface {
	Create(ctx context.Context, t *domain.Transfer, fileIDs []string) (*domain.Transfer, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Transfer, error)
	GetByID(ctx context.Context, id string) (*domain.Transfer, error)
	ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*domain.Transfer, error)
	GetFileIDs(ctx context.Context, transferID string) ([]string, error)
	GetFileInfos(ctx context.Context, transferID string) ([]*repository.FileInfo, error)
	AttemptFileDownload(ctx context.Context, transferID, fileID string, maxDownloads int) (bool, error)
	GetFileDownloadCounts(ctx context.Context, transferID string) (map[string]int, error)
	IncrementDownloads(ctx context.Context, id string) error
	Revoke(ctx context.Context, id, ownerID string) error
	GetTransfersNeedingReminder(ctx context.Context) ([]*domain.Transfer, error)
	MarkReminderSent(ctx context.Context, id string) error
	UpdateRecipientEmail(ctx context.Context, id, ownerID, recipientEmail string) error
	StoreRecipientToken(ctx context.Context, tok *domain.RecipientToken) error
	GetRecipientTokenByHash(ctx context.Context, tokenHash string) (*domain.RecipientToken, error)
	DeleteRecipientToken(ctx context.Context, transferID, email string) error
}

// TransferService handles business logic for creating and accessing transfers.
type TransferService struct {
	repo transferRepository
	cfg  *config.Config
	js   *jetstream.Client
	log  *zap.Logger
}

func New(repo *repository.TransferRepository, cfg *config.Config, js *jetstream.Client, log *zap.Logger) *TransferService {
	return &TransferService{repo: repo, cfg: cfg, js: js, log: log}
}

// CreateParams carries creation inputs from the handler.
type CreateParams struct {
	OwnerID          string
	SenderEmail      string // email of the creating user, stored for display to recipients
	Name             string
	Description      string
	FileIDs          []string
	RecipientEmail   string
	Password         string // empty = no password
	MaxDownloads     int
	ViewOnly         bool          // true = serve files inline; no download button for recipients
	NotifyOnDownload bool          // true = email the owner when a recipient downloads a file
	ExpiresIn        time.Duration // must be > 0 and <= 90 days
	ClientIP         string        // for audit log
}

// CreateResult is returned to the handler after successful creation.
type CreateResult struct {
	Transfer *domain.Transfer
	FileIDs  []string
}

const maxExpiresIn = 90 * 24 * time.Hour // 3 months

func (s *TransferService) Create(ctx context.Context, p CreateParams) (*CreateResult, error) {
	if strings.TrimSpace(p.Name) == "" {
		return nil, apperrors.Validation("name is required")
	}
	if len(p.FileIDs) == 0 {
		return nil, apperrors.Validation("at least one file is required")
	}
	if p.ExpiresIn <= 0 {
		return nil, apperrors.Validation("expiry is required")
	}
	if p.ExpiresIn > maxExpiresIn {
		return nil, apperrors.Validation("expiry cannot exceed 90 days")
	}

	slug, err := crypto.RandomToken(defaultSlugBytes)
	if err != nil {
		return nil, apperrors.Internal("generate slug", err)
	}

	t := &domain.Transfer{
		OwnerID:          p.OwnerID,
		SenderEmail:      p.SenderEmail,
		Name:             strings.TrimSpace(p.Name),
		Description:      strings.TrimSpace(p.Description),
		RecipientEmail:   p.RecipientEmail,
		Slug:             slug,
		MaxDownloads:     p.MaxDownloads,
		ViewOnly:         p.ViewOnly,
		NotifyOnDownload: p.NotifyOnDownload,
	}

	if p.Password != "" {
		hash, err := crypto.HashPassword(p.Password, s.cfg.App.Pepper)
		if err != nil {
			return nil, apperrors.Internal("hash transfer password", err)
		}
		t.PasswordHash = hash
	}

	if p.ExpiresIn > 0 {
		exp := time.Now().Add(p.ExpiresIn)
		t.ExpiresAt = &exp
	}

	created, err := s.repo.Create(ctx, t, p.FileIDs)
	if err != nil {
		return nil, err
	}

	s.publishAudit(ctx, "transfer.created", p.OwnerID, created.ID, p.ClientIP)
	s.publishEmailNotification(ctx, created)

	return &CreateResult{Transfer: created, FileIDs: p.FileIDs}, nil
}

// publishAudit publishes an audit event asynchronously; failure is logged, not returned.
func (s *TransferService) publishAudit(ctx context.Context, action, ownerID, transferID, clientIP string) {
	if s.js == nil {
		return
	}
	ev := map[string]any{
		"action":      action,
		"user_id":     ownerID,
		"transfer_id": transferID,
		"client_ip":   clientIP,
		"success":     true,
		"timestamp":   time.Now(),
	}
	go func() {
		if err := s.js.Publish(ctx, "AUDIT.transfer", ev); err != nil {
			s.log.Warn("failed to publish audit event", zap.Error(err))
		}
	}()
}

// publishEmailNotification publishes a transfer_received email event.
// For each recipient email, a unique magic-link token is generated and embedded
// in the download URL (?rt=<token>), so each recipient gets a personalised
// secure link that does not require a password.
func (s *TransferService) publishEmailNotification(ctx context.Context, t *domain.Transfer) {
	if s.js == nil || t.RecipientEmail == "" {
		return
	}

	// Support comma-separated multiple recipients.
	var recipients []string
	for _, r := range strings.Split(t.RecipientEmail, ",") {
		if addr := strings.TrimSpace(r); addr != "" {
			recipients = append(recipients, addr)
		}
	}
	if len(recipients) == 0 {
		return
	}

	var expiresAt string
	var tokenExpiry time.Time
	if t.ExpiresAt != nil {
		expiresAt = t.ExpiresAt.Format(time.RFC1123)
		tokenExpiry = *t.ExpiresAt
	} else {
		tokenExpiry = time.Now().Add(30 * 24 * time.Hour)
	}

	senderName := t.SenderEmail
	if senderName == "" {
		senderName = "a TenzoShare user"
	}

	// Publish one email per recipient so each gets their own personalised link.
	for _, email := range recipients {
		rawToken, tokenHash, err := s.generateTokenHash()
		if err != nil {
			s.log.Warn("failed to generate recipient token", zap.String("email", email), zap.Error(err))
			continue
		}
		tok := &domain.RecipientToken{
			TransferID: t.ID,
			Email:      email,
			TokenHash:  tokenHash,
			ExpiresAt:  tokenExpiry,
		}
		if err := s.repo.StoreRecipientToken(ctx, tok); err != nil {
			s.log.Warn("failed to store recipient token", zap.String("email", email), zap.Error(err))
			continue
		}
		downloadURL := s.cfg.App.BaseURL + "/t/" + t.Slug + "?rt=" + rawToken
		data, _ := json.Marshal(map[string]any{
			"SenderName":  senderName,
			"Slug":        t.Slug,
			"Title":       t.Name,
			"Message":     t.Description,
			"DownloadURL": downloadURL,
			"ExpiresAt":   expiresAt,
			"HasPassword": t.PasswordHash != "",
		})
		addr := email // capture for goroutine
		ev := map[string]any{
			"type": "transfer_received",
			"to":   []string{addr},
			"data": json.RawMessage(data),
		}
		go func() {
			if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
				s.log.Warn("failed to publish email notification", zap.Error(err))
			}
		}()
	}
}

// publishDownloadNotificationEmail notifies the transfer owner when a file is downloaded.
// Only fires when the owner has an email address and NATS is available.
// Best-effort: errors are only logged.
func (s *TransferService) publishDownloadNotificationEmail(ctx context.Context, t *domain.Transfer, fileID string) {
	if s.js == nil || t.SenderEmail == "" || !t.NotifyOnDownload {
		return
	}

	downloadURL := s.cfg.App.BaseURL + "/t/" + t.Slug
	recipientLabel := t.RecipientEmail
	if recipientLabel == "" {
		recipientLabel = "a public link visitor"
	}

	data, _ := json.Marshal(map[string]any{
		"Title":          t.Name,
		"Slug":           t.Slug,
		"RecipientEmail": recipientLabel,
		"DownloadedAt":   time.Now().UTC().Format(time.RFC1123),
		"DownloadURL":    downloadURL,
	})

	ev := map[string]any{
		"type": "download_notification",
		"to":   []string{t.SenderEmail},
		"data": json.RawMessage(data),
	}
	go func() {
		if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
			s.log.Warn("failed to publish download_notification email", zap.Error(err))
		}
	}()
}

// AccessParams carries the slug and optional password for public transfer access.
type AccessParams struct {
	Slug     string
	Password string // empty if no password provided
}

// AccessResult is returned by Validate (download page info) and AttemptFileDownload.
type AccessResult struct {
	Transfer           *domain.Transfer
	FileIDs            []string
	FileInfos          []*repository.FileInfo // populated by Validate; nil for AttemptFileDownload
	FileDownloadCounts map[string]int         // populated by Validate; nil for AttemptFileDownload
}

// AttemptFileDownloadParams carries the parameters for the per-file download endpoint.
type AttemptFileDownloadParams struct {
	Slug     string
	FileID   string
	Password string
	ClientIP string // for audit log
}

// AttemptFileDownload validates the transfer and atomically checks+increments the
// per-file download counter. This is the authoritative access gate for the actual
// file download endpoint. Per-file enforcement means downloading file A cannot
// consume quota for file B.
func (s *TransferService) AttemptFileDownload(ctx context.Context, p AttemptFileDownloadParams) (*AccessResult, error) {
	t, err := s.repo.GetBySlug(ctx, p.Slug)
	if err != nil {
		return nil, err
	}

	if t.IsRevoked {
		return nil, apperrors.Forbidden("this transfer has been revoked")
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return nil, apperrors.Forbidden("this transfer has expired")
	}

	if t.PasswordHash != "" {
		if p.Password == "" {
			return nil, apperrors.Unauthorized("password required")
		}
		ok, err := crypto.VerifyPassword(p.Password, t.PasswordHash, s.cfg.App.Pepper)
		if err != nil {
			return nil, apperrors.Internal("verify transfer password", err)
		}
		if !ok {
			return nil, apperrors.Unauthorized("incorrect password")
		}
	}

	fileIDs, err := s.repo.GetFileIDs(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	// Confirm the requested file belongs to this transfer.
	found := false
	for _, fid := range fileIDs {
		if fid == p.FileID {
			found = true
			break
		}
	}
	if !found {
		return nil, apperrors.NotFound("file not found in this transfer")
	}

	// Atomically check and increment the per-file download counter.
	// Returns false (without error) when the file's individual limit is reached.
	if t.MaxDownloads > 0 {
		allowed, err := s.repo.AttemptFileDownload(ctx, t.ID, p.FileID, t.MaxDownloads)
		if err != nil {
			return nil, apperrors.Internal("check file download limit", err)
		}
		if !allowed {
			return nil, apperrors.Forbidden("download limit reached for this file")
		}
	}

	// Increment global counter for display purposes (non-blocking, best-effort).
	go func() {
		if err := s.repo.IncrementDownloads(context.Background(), t.ID); err != nil {
			s.log.Warn("failed to increment download count", zap.String("transfer_id", t.ID), zap.Error(err))
		}
	}()

	s.publishAudit(ctx, "transfer.downloaded", t.OwnerID, t.ID, p.ClientIP)
	s.publishDownloadNotificationEmail(ctx, t, p.FileID)

	return &AccessResult{Transfer: t, FileIDs: fileIDs}, nil
}

// Validate checks a transfer is accessible (revoked/expired/password) without
// modifying any state. Used by GET /t/:slug (download page) so the page always
// loads — individual file exhaustion is shown via FileDownloadCounts.
func (s *TransferService) Validate(ctx context.Context, p AccessParams) (*AccessResult, error) {
	t, err := s.repo.GetBySlug(ctx, p.Slug)
	if err != nil {
		return nil, err
	}

	if t.IsRevoked {
		return nil, apperrors.Forbidden("this transfer has been revoked")
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return nil, apperrors.Forbidden("this transfer has expired")
	}

	if t.PasswordHash != "" {
		if p.Password == "" {
			return nil, apperrors.Unauthorized("password required")
		}
		ok, err := crypto.VerifyPassword(p.Password, t.PasswordHash, s.cfg.App.Pepper)
		if err != nil {
			return nil, apperrors.Internal("verify transfer password", err)
		}
		if !ok {
			return nil, apperrors.Unauthorized("incorrect password")
		}
	}

	fileIDs, err := s.repo.GetFileIDs(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	fileInfos, err := s.repo.GetFileInfos(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	// Fetch per-file download counts so the download UI can show per-file status.
	var fileCounts map[string]int
	if t.MaxDownloads > 0 {
		fileCounts, _ = s.repo.GetFileDownloadCounts(ctx, t.ID)
		if fileCounts == nil {
			fileCounts = make(map[string]int)
		}
	}

	return &AccessResult{Transfer: t, FileIDs: fileIDs, FileInfos: fileInfos, FileDownloadCounts: fileCounts}, nil
}

func (s *TransferService) Revoke(ctx context.Context, id, ownerID string) error {
	// Fetch before revoking so we can notify the recipient.
	t, _ := s.repo.GetByID(ctx, id)
	err := s.repo.Revoke(ctx, id, ownerID)
	if err == nil {
		s.publishAudit(ctx, "transfer.revoked", ownerID, id, "")
		if t != nil {
			s.publishRevokedEmail(ctx, t)
		}
	}
	return err
}

func (s *TransferService) Get(ctx context.Context, id, ownerID string) (*domain.Transfer, []string, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if t.OwnerID != ownerID {
		return nil, nil, apperrors.Forbidden("access denied")
	}
	fileIDs, err := s.repo.GetFileIDs(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return t, fileIDs, nil
}

// GetByID fetches a transfer without enforcing ownership — callers must do their own ACL check.
func (s *TransferService) GetByID(ctx context.Context, id string) (*domain.Transfer, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TransferService) List(ctx context.Context, ownerID string, limit, offset int) ([]*domain.Transfer, error) {
	return s.repo.ListByOwner(ctx, ownerID, limit, offset)
}

// publishRevokedEmail notifies the transfer recipient (if any) that the transfer was revoked.
func (s *TransferService) publishRevokedEmail(ctx context.Context, t *domain.Transfer) {
	if s.js == nil || t.RecipientEmail == "" {
		return
	}
	data, _ := json.Marshal(map[string]any{
		"Title":       t.Name,
		"SenderEmail": t.SenderEmail,
	})
	ev := map[string]any{
		"type": "transfer_revoked",
		"to":   []string{t.RecipientEmail},
		"data": json.RawMessage(data),
	}
	go func() {
		if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
			s.log.Warn("failed to publish transfer_revoked email", zap.Error(err))
		}
	}()
}

// UpdateRecipients replaces the recipient list for a transfer (owner only).
// emails may be empty to make the transfer public-link-only.
func (s *TransferService) UpdateRecipients(ctx context.Context, id, ownerID string, emails []string) (*domain.Transfer, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.OwnerID != ownerID {
		return nil, apperrors.Forbidden("access denied")
	}

	// Deduplicate and normalise.
	seen := map[string]struct{}{}
	var unique []string
	for _, e := range emails {
		if e = strings.TrimSpace(e); e != "" {
			if _, dup := seen[e]; !dup {
				seen[e] = struct{}{}
				unique = append(unique, e)
			}
		}
	}
	recipientEmail := strings.Join(unique, ",")

	if err := s.repo.UpdateRecipientEmail(ctx, id, ownerID, recipientEmail); err != nil {
		return nil, err
	}
	t.RecipientEmail = recipientEmail
	return t, nil
}

// ResendNotification re-publishes the transfer_received email to all current recipients.
// Only the owner may call this; the transfer must be active (not revoked/expired).
func (s *TransferService) ResendNotification(ctx context.Context, id, ownerID string) error {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t.OwnerID != ownerID {
		return apperrors.Forbidden("access denied")
	}
	if t.IsRevoked {
		return apperrors.BadRequest("cannot resend: transfer is revoked")
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return apperrors.BadRequest("cannot resend: transfer has expired")
	}
	if t.RecipientEmail == "" {
		return apperrors.BadRequest("no recipients to notify")
	}
	s.publishEmailNotification(ctx, t)
	return nil
}

// SendExpiryReminders queries transfers expiring within 24 hours that haven't
// received a reminder yet and publishes transfer_expiry_reminder email events.
// Intended to be called from a background goroutine (hourly).
func (s *TransferService) SendExpiryReminders(ctx context.Context) {
	if s.js == nil {
		return
	}
	transfers, err := s.repo.GetTransfersNeedingReminder(ctx)
	if err != nil {
		s.log.Warn("failed to fetch transfers needing reminder", zap.Error(err))
		return
	}
	for _, t := range transfers {
		recipient := t.RecipientEmail
		if recipient == "" {
			// Public link — notify the sender instead.
			recipient = t.SenderEmail
		}
		if recipient == "" {
			continue
		}

		downloadURL := s.cfg.App.BaseURL + "/t/" + t.Slug
		var expiresAt string
		if t.ExpiresAt != nil {
			expiresAt = t.ExpiresAt.Format(time.RFC1123)
		}
		data, _ := json.Marshal(map[string]any{
			"Title":       t.Name,
			"DownloadURL": downloadURL,
			"ExpiresAt":   expiresAt,
		})
		ev := map[string]any{
			"type": "transfer_expiry_reminder",
			"to":   []string{recipient},
			"data": json.RawMessage(data),
		}
		tID := t.ID
		if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
			s.log.Warn("failed to publish expiry reminder email", zap.String("transfer_id", tID), zap.Error(err))
			continue
		}
		if err := s.repo.MarkReminderSent(ctx, tID); err != nil {
			s.log.Warn("failed to mark reminder sent", zap.String("transfer_id", tID), zap.Error(err))
		}
	}
}

// generateTokenHash generates a new random recipient token.
// Returns (rawToken, hexSHA256Hash, error).
// rawToken is the base64url value embedded in email links.
// hexSHA256Hash is stored in the DB.
func (s *TransferService) generateTokenHash() (string, string, error) {
	rawToken, err := crypto.RandomToken(32)
	if err != nil {
		return "", "", err
	}
	h := sha256.Sum256([]byte(rawToken))
	return rawToken, hex.EncodeToString(h[:]), nil
}

// ValidateRecipientToken validates a raw magic-link token (?rt=) against the DB.
// On success it returns the full AccessResult (bypassing any password check).
// Returns Unauthorized if the token is invalid or expired.
func (s *TransferService) ValidateRecipientToken(ctx context.Context, slug, rawToken string) (*AccessResult, error) {
	h := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(h[:])

	tok, err := s.repo.GetRecipientTokenByHash(ctx, tokenHash)
	if err != nil {
		// NotFound → return a generic invalid message to avoid oracle attacks.
		return nil, apperrors.Unauthorized("invalid or expired access link — request a new one")
	}

	if time.Now().After(tok.ExpiresAt) {
		return nil, apperrors.Unauthorized("access link has expired — request a new one")
	}

	// Fetch the transfer and validate its live state.
	t, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if t.ID != tok.TransferID {
		return nil, apperrors.Unauthorized("invalid or expired access link — request a new one")
	}
	if t.IsRevoked {
		return nil, apperrors.Forbidden("this transfer has been revoked")
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return nil, apperrors.Forbidden("this transfer has expired")
	}

	fileIDs, err := s.repo.GetFileIDs(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	fileInfos, err := s.repo.GetFileInfos(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	var fileCounts map[string]int
	if t.MaxDownloads > 0 {
		fileCounts, _ = s.repo.GetFileDownloadCounts(ctx, t.ID)
		if fileCounts == nil {
			fileCounts = make(map[string]int)
		}
	}
	return &AccessResult{Transfer: t, FileIDs: fileIDs, FileInfos: fileInfos, FileDownloadCounts: fileCounts}, nil
}

// RegenerateRecipientToken generates a new magic-link token for a recipient email
// and re-sends the transfer_received email. Called when a recipient's link has expired.
// Rate limiting is enforced at the handler layer.
func (s *TransferService) RegenerateRecipientToken(ctx context.Context, slug, email string) error {
	t, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if t.IsRevoked {
		return apperrors.Forbidden("this transfer has been revoked")
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return apperrors.Forbidden("this transfer has expired")
	}

	// Verify the email is among the original recipients.
	found := false
	for _, r := range strings.Split(t.RecipientEmail, ",") {
		if strings.EqualFold(strings.TrimSpace(r), strings.TrimSpace(email)) {
			found = true
			break
		}
	}
	if !found {
		// Return success anyway — don't leak which emails are recipients.
		return nil
	}

	var tokenExpiry time.Time
	if t.ExpiresAt != nil {
		tokenExpiry = *t.ExpiresAt
	} else {
		tokenExpiry = time.Now().Add(30 * 24 * time.Hour)
	}

	rawToken, tokenHash, err := s.generateTokenHash()
	if err != nil {
		return apperrors.Internal("generate recipient token", err)
	}
	tok := &domain.RecipientToken{
		TransferID: t.ID,
		Email:      email,
		TokenHash:  tokenHash,
		ExpiresAt:  tokenExpiry,
	}
	if err := s.repo.StoreRecipientToken(ctx, tok); err != nil {
		return err
	}

	if s.js == nil {
		return nil
	}
	downloadURL := s.cfg.App.BaseURL + "/t/" + t.Slug + "?rt=" + rawToken
	var expiresAt string
	if t.ExpiresAt != nil {
		expiresAt = t.ExpiresAt.Format(time.RFC1123)
	}
	senderName := t.SenderEmail
	if senderName == "" {
		senderName = "a TenzoShare user"
	}
	data, _ := json.Marshal(map[string]any{
		"SenderName":  senderName,
		"Slug":        t.Slug,
		"Title":       t.Name,
		"Message":     t.Description,
		"DownloadURL": downloadURL,
		"ExpiresAt":   expiresAt,
		"HasPassword": t.PasswordHash != "",
	})
	ev := map[string]any{
		"type": "transfer_received",
		"to":   []string{email},
		"data": json.RawMessage(data),
	}
	go func() {
		if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
			s.log.Warn("failed to publish resend access email", zap.Error(err))
		}
	}()
	return nil
}
