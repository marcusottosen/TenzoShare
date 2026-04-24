package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tenzoshare/tenzoshare/services/transfer/internal/domain"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

type TransferRepository struct {
	db *pgxpool.Pool
}

func NewTransferRepository(db *pgxpool.Pool) *TransferRepository {
	return &TransferRepository{db: db}
}

// Create inserts a transfer record and associates fileIDs in a transaction.
func (r *TransferRepository) Create(ctx context.Context, t *domain.Transfer, fileIDs []string) (*domain.Transfer, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, apperrors.Internal("begin transaction", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var out domain.Transfer
	err = tx.QueryRow(ctx, `
		INSERT INTO transfer.transfers
			(owner_id, recipient_email, slug, password_hash, max_downloads, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, owner_id, recipient_email, slug, password_hash, max_downloads, download_count, expires_at, is_revoked, created_at
	`, t.OwnerID, t.RecipientEmail, t.Slug, t.PasswordHash, t.MaxDownloads, t.ExpiresAt).Scan(
		&out.ID, &out.OwnerID, &out.RecipientEmail, &out.Slug, &out.PasswordHash,
		&out.MaxDownloads, &out.DownloadCount, &out.ExpiresAt, &out.IsRevoked, &out.CreatedAt,
	)
	if err != nil {
		return nil, apperrors.Internal("insert transfer", err)
	}

	for _, fid := range fileIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO transfer.transfer_files (transfer_id, file_id) VALUES ($1, $2)
		`, out.ID, fid); err != nil {
			return nil, apperrors.Internal(fmt.Sprintf("associate file %s", fid), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, apperrors.Internal("commit transaction", err)
	}
	return &out, nil
}

// GetBySlug looks up a transfer by its public slug, used for downloads.
func (r *TransferRepository) GetBySlug(ctx context.Context, slug string) (*domain.Transfer, error) {
	var t domain.Transfer
	err := r.db.QueryRow(ctx, `
		SELECT id, owner_id, recipient_email, slug, password_hash, max_downloads, download_count, expires_at, is_revoked, created_at
		FROM transfer.transfers
		WHERE slug = $1
	`, slug).Scan(
		&t.ID, &t.OwnerID, &t.RecipientEmail, &t.Slug, &t.PasswordHash,
		&t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.IsRevoked, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("transfer not found")
		}
		return nil, apperrors.Internal("get transfer by slug", err)
	}
	return &t, nil
}

// GetByID looks up a transfer by primary key (owner management).
func (r *TransferRepository) GetByID(ctx context.Context, id string) (*domain.Transfer, error) {
	var t domain.Transfer
	err := r.db.QueryRow(ctx, `
		SELECT id, owner_id, recipient_email, slug, password_hash, max_downloads, download_count, expires_at, is_revoked, created_at
		FROM transfer.transfers
		WHERE id = $1
	`, id).Scan(
		&t.ID, &t.OwnerID, &t.RecipientEmail, &t.Slug, &t.PasswordHash,
		&t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.IsRevoked, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("transfer not found")
		}
		return nil, apperrors.Internal("get transfer", err)
	}
	return &t, nil
}

// ListByOwner returns all transfers belonging to ownerID (newest first).
func (r *TransferRepository) ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*domain.Transfer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_id, recipient_email, slug, password_hash, max_downloads, download_count, expires_at, is_revoked, created_at
		FROM transfer.transfers
		WHERE owner_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, ownerID, limit, offset)
	if err != nil {
		return nil, apperrors.Internal("list transfers", err)
	}
	defer rows.Close()

	var transfers []*domain.Transfer
	for rows.Next() {
		var t domain.Transfer
		if err := rows.Scan(
			&t.ID, &t.OwnerID, &t.RecipientEmail, &t.Slug, &t.PasswordHash,
			&t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.IsRevoked, &t.CreatedAt,
		); err != nil {
			return nil, apperrors.Internal("scan transfer", err)
		}
		transfers = append(transfers, &t)
	}
	return transfers, rows.Err()
}

// IncrementDownloads atomically bumps the download counter.
func (r *TransferRepository) IncrementDownloads(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE transfer.transfers SET download_count = download_count + 1 WHERE id = $1
	`, id)
	if err != nil {
		return apperrors.Internal("increment download count", err)
	}
	return nil
}

// Revoke marks a transfer as revoked so it can no longer be downloaded.
func (r *TransferRepository) Revoke(ctx context.Context, id, ownerID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE transfer.transfers SET is_revoked = TRUE WHERE id = $1 AND owner_id = $2
	`, id, ownerID)
	if err != nil {
		return apperrors.Internal("revoke transfer", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("transfer not found")
	}
	return nil
}

// GetFileIDs returns the file UUIDs attached to a transfer.
func (r *TransferRepository) GetFileIDs(ctx context.Context, transferID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT file_id FROM transfer.transfer_files WHERE transfer_id = $1
	`, transferID)
	if err != nil {
		return nil, apperrors.Internal("get transfer files", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var fid string
		if err := rows.Scan(&fid); err != nil {
			return nil, apperrors.Internal("scan file id", err)
		}
		ids = append(ids, fid)
	}
	return ids, rows.Err()
}
