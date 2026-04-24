package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/transfer/internal/repository"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

const defaultSlugBytes = 12 // 96-bit slug → 16 URL-safe base64 chars

// TransferService handles business logic for creating and accessing transfers.
type TransferService struct {
	repo *repository.TransferRepository
	cfg  *config.Config
	log  *zap.Logger
}

func New(repo *repository.TransferRepository, cfg *config.Config, log *zap.Logger) *TransferService {
	return &TransferService{repo: repo, cfg: cfg, log: log}
}

// CreateParams carries creation inputs from the handler.
type CreateParams struct {
	OwnerID        string
	FileIDs        []string
	RecipientEmail string
	Password       string // empty = no password
	MaxDownloads   int
	ExpiresIn      time.Duration // 0 = no expiry
}

// CreateResult is returned to the handler after successful creation.
type CreateResult struct {
	Transfer *domain.Transfer
	FileIDs  []string
}

func (s *TransferService) Create(ctx context.Context, p CreateParams) (*CreateResult, error) {
	if len(p.FileIDs) == 0 {
		return nil, apperrors.Validation("at least one file is required")
	}

	slug, err := crypto.RandomToken(defaultSlugBytes)
	if err != nil {
		return nil, apperrors.Internal("generate slug", err)
	}

	t := &domain.Transfer{
		OwnerID:        p.OwnerID,
		RecipientEmail: p.RecipientEmail,
		Slug:           slug,
		MaxDownloads:   p.MaxDownloads,
	}

	if p.Password != "" {
		hash, err := crypto.HashPassword(p.Password, s.cfg.App.BaseURL)
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

	return &CreateResult{Transfer: created, FileIDs: p.FileIDs}, nil
}

// Access validates a transfer is reachable and (if protected) checks the password.
// Returns the transfer and its file IDs on success.
type AccessParams struct {
	Slug     string
	Password string // empty if no password provided by downloader
}

type AccessResult struct {
	Transfer *domain.Transfer
	FileIDs  []string
}

func (s *TransferService) Access(ctx context.Context, p AccessParams) (*AccessResult, error) {
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
	if t.MaxDownloads > 0 && t.DownloadCount >= t.MaxDownloads {
		return nil, apperrors.Forbidden("download limit reached")
	}

	if t.PasswordHash != "" {
		if p.Password == "" {
			return nil, apperrors.Unauthorized("password required")
		}
		ok, err := crypto.VerifyPassword(p.Password, t.PasswordHash, s.cfg.App.BaseURL)
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

	// Bump counter in background to keep the response fast.
	go func() {
		if err := s.repo.IncrementDownloads(context.Background(), t.ID); err != nil {
			s.log.Warn("failed to increment download count", zap.String("transfer_id", t.ID), zap.Error(err))
		}
	}()

	return &AccessResult{Transfer: t, FileIDs: fileIDs}, nil
}

func (s *TransferService) Revoke(ctx context.Context, id, ownerID string) error {
	return s.repo.Revoke(ctx, id, ownerID)
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

func (s *TransferService) List(ctx context.Context, ownerID string, limit, offset int) ([]*domain.Transfer, error) {
	return s.repo.ListByOwner(ctx, ownerID, limit, offset)
}
