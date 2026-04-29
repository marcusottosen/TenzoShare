package service

import (
	"context"
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
	OwnerID        string
	SenderEmail    string // email of the creating user, stored for display to recipients
	Name           string
	Description    string
	FileIDs        []string
	RecipientEmail string
	Password       string // empty = no password
	MaxDownloads   int
	ExpiresIn      time.Duration // must be > 0 and <= 90 days
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
		OwnerID:        p.OwnerID,
		SenderEmail:    p.SenderEmail,
		Name:           strings.TrimSpace(p.Name),
		Description:    strings.TrimSpace(p.Description),
		RecipientEmail: p.RecipientEmail,
		Slug:           slug,
		MaxDownloads:   p.MaxDownloads,
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

	s.publishAudit(ctx, "transfer.created", p.OwnerID, created.ID)
	s.publishEmailNotification(ctx, created)

	return &CreateResult{Transfer: created, FileIDs: p.FileIDs}, nil
}

// publishAudit publishes an audit event asynchronously; failure is logged, not returned.
func (s *TransferService) publishAudit(ctx context.Context, action, ownerID, transferID string) {
	if s.js == nil {
		return
	}
	ev := map[string]any{
		"action":      action,
		"user_id":     ownerID,
		"transfer_id": transferID,
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
func (s *TransferService) publishEmailNotification(ctx context.Context, t *domain.Transfer) {
	if s.js == nil || t.RecipientEmail == "" {
		return
	}

	downloadURL := s.cfg.App.BaseURL + "/t/" + t.Slug
	var expiresAt string
	if t.ExpiresAt != nil {
		expiresAt = t.ExpiresAt.Format(time.RFC1123)
	}

	data, _ := json.Marshal(map[string]any{
		"SenderName":  "a TenzoShare user",
		"Title":       t.Slug, // will be replaced with real title field in future
		"DownloadURL": downloadURL,
		"ExpiresAt":   expiresAt,
		"HasPassword": t.PasswordHash != "",
	})

	ev := map[string]any{
		"type": "transfer_received",
		"to":   []string{t.RecipientEmail},
		"data": json.RawMessage(data),
	}
	go func() {
		if err := s.js.Publish(ctx, "NOTIFICATIONS.email", ev); err != nil {
			s.log.Warn("failed to publish email notification", zap.Error(err))
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
	err := s.repo.Revoke(ctx, id, ownerID)
	if err == nil {
		s.publishAudit(ctx, "transfer.revoked", ownerID, id)
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
