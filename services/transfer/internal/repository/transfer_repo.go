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
			(owner_id, sender_email, name, description, recipient_email, slug, password_hash, max_downloads, expires_at, view_only, notify_on_download)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, owner_id, sender_email, name, description, recipient_email, slug, password_hash, max_downloads, download_count, expires_at, is_revoked, created_at, view_only, notify_on_download
	`, t.OwnerID, t.SenderEmail, t.Name, t.Description, t.RecipientEmail, t.Slug, t.PasswordHash, t.MaxDownloads, t.ExpiresAt, t.ViewOnly, t.NotifyOnDownload).Scan(
		&out.ID, &out.OwnerID, &out.SenderEmail, &out.Name, &out.Description, &out.RecipientEmail, &out.Slug, &out.PasswordHash,
		&out.MaxDownloads, &out.DownloadCount, &out.ExpiresAt, &out.IsRevoked, &out.CreatedAt, &out.ViewOnly, &out.NotifyOnDownload,
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
		SELECT t.id, t.owner_id, t.sender_email, t.name, t.description, t.recipient_email, t.slug, t.password_hash,
		       t.max_downloads, t.download_count, t.expires_at, t.is_revoked, t.created_at, t.view_only, t.notify_on_download,
		       COALESCE((SELECT count(*) FROM transfer.transfer_files tf WHERE tf.transfer_id = t.id), 0) AS file_count,
		       COALESCE((SELECT sum(f.size_bytes) FROM transfer.transfer_files tf JOIN storage.files f ON f.id = tf.file_id WHERE tf.transfer_id = t.id AND f.deleted_at IS NULL), 0) AS total_size_bytes,
		       (t.max_downloads > 0 AND NOT EXISTS (
		           SELECT 1 FROM transfer.transfer_files tf
		           LEFT JOIN transfer.file_download_counts fdc
		               ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		           WHERE tf.transfer_id = t.id
		             AND COALESCE(fdc.count, 0) < t.max_downloads
		       )) AS is_exhausted
		FROM transfer.transfers t
		WHERE t.slug = $1
	`, slug).Scan(
		&t.ID, &t.OwnerID, &t.SenderEmail, &t.Name, &t.Description, &t.RecipientEmail, &t.Slug, &t.PasswordHash,
		&t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.IsRevoked, &t.CreatedAt, &t.ViewOnly, &t.NotifyOnDownload, &t.FileCount, &t.TotalSizeBytes, &t.IsExhausted,
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
		SELECT t.id, t.owner_id, t.sender_email, t.name, t.description, t.recipient_email, t.slug, t.password_hash,
		       t.max_downloads, t.download_count, t.expires_at, t.is_revoked, t.created_at, t.view_only, t.notify_on_download,
		       COALESCE((SELECT count(*) FROM transfer.transfer_files tf WHERE tf.transfer_id = t.id), 0) AS file_count,
		       COALESCE((SELECT sum(f.size_bytes) FROM transfer.transfer_files tf JOIN storage.files f ON f.id = tf.file_id WHERE tf.transfer_id = t.id AND f.deleted_at IS NULL), 0) AS total_size_bytes,
		       (t.max_downloads > 0 AND NOT EXISTS (
		           SELECT 1 FROM transfer.transfer_files tf
		           LEFT JOIN transfer.file_download_counts fdc
		               ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		           WHERE tf.transfer_id = t.id
		             AND COALESCE(fdc.count, 0) < t.max_downloads
		       )) AS is_exhausted
		FROM transfer.transfers t
		WHERE t.id = $1
	`, id).Scan(
		&t.ID, &t.OwnerID, &t.SenderEmail, &t.Name, &t.Description, &t.RecipientEmail, &t.Slug, &t.PasswordHash,
		&t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.IsRevoked, &t.CreatedAt, &t.ViewOnly, &t.NotifyOnDownload, &t.FileCount, &t.TotalSizeBytes, &t.IsExhausted,
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
		SELECT t.id, t.owner_id, t.sender_email, t.name, t.description, t.recipient_email, t.slug, t.password_hash,
		       t.max_downloads, t.download_count, t.expires_at, t.is_revoked, t.created_at, t.view_only, t.notify_on_download,
		       COALESCE((SELECT count(*) FROM transfer.transfer_files tf WHERE tf.transfer_id = t.id), 0) AS file_count,
		       COALESCE((SELECT sum(f.size_bytes) FROM transfer.transfer_files tf JOIN storage.files f ON f.id = tf.file_id WHERE tf.transfer_id = t.id AND f.deleted_at IS NULL), 0) AS total_size_bytes,
		       (t.max_downloads > 0 AND NOT EXISTS (
		           SELECT 1 FROM transfer.transfer_files tf
		           LEFT JOIN transfer.file_download_counts fdc
		               ON fdc.transfer_id = tf.transfer_id AND fdc.file_id = tf.file_id
		           WHERE tf.transfer_id = t.id
		             AND COALESCE(fdc.count, 0) < t.max_downloads
		       )) AS is_exhausted
		FROM transfer.transfers t
		WHERE t.owner_id = $1
		ORDER BY t.created_at DESC
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
			&t.ID, &t.OwnerID, &t.SenderEmail, &t.Name, &t.Description, &t.RecipientEmail, &t.Slug, &t.PasswordHash,
			&t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.IsRevoked, &t.CreatedAt, &t.ViewOnly, &t.NotifyOnDownload, &t.FileCount, &t.TotalSizeBytes, &t.IsExhausted,
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

// FileInfo holds per-file metadata for display on the download page.
// DeleteReason is empty for available files; non-empty values are:
//
//	"owner_deleted"      – the file owner deleted their own file
//	"admin_purge"        – an administrator force-deleted the file
//	"retention_expired"  – auto-purged by the retention policy
//	"orphan_expired"     – auto-purged as an orphaned file
type FileInfo struct {
	ID           string
	Filename     string
	ContentType  string
	SizeBytes    int64
	DeleteReason string // empty = available
}

// GetFileInfos returns filename/size/type metadata for every file in a transfer,
// including deleted files (so the UI can show them as unavailable with an appropriate reason).
func (r *TransferRepository) GetFileInfos(ctx context.Context, transferID string) ([]*FileInfo, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sf.id, sf.filename, sf.content_type, sf.size_bytes,
		       CASE
		           WHEN sf.deleted_at IS NULL THEN ''
		           ELSE COALESCE(pl.reason, 'owner_deleted')
		       END AS delete_reason
		FROM transfer.transfer_files tf
		JOIN storage.files sf ON sf.id::text = tf.file_id::text
		LEFT JOIN LATERAL (
		    SELECT reason
		    FROM storage.file_purge_log
		    WHERE file_id = sf.id
		    ORDER BY purged_at DESC
		    LIMIT 1
		) pl ON sf.deleted_at IS NOT NULL
		WHERE tf.transfer_id = $1
		ORDER BY sf.filename
	`, transferID)
	if err != nil {
		return nil, apperrors.Internal("get file infos", err)
	}
	defer rows.Close()

	var infos []*FileInfo
	for rows.Next() {
		var f FileInfo
		if err := rows.Scan(&f.ID, &f.Filename, &f.ContentType, &f.SizeBytes, &f.DeleteReason); err != nil {
			return nil, apperrors.Internal("scan file info", err)
		}
		infos = append(infos, &f)
	}
	return infos, rows.Err()
}

// AttemptFileDownload atomically checks and increments the per-file download counter.
// Returns (true, nil) if the download is allowed (counter was incremented).
// Returns (false, nil) if the per-file limit has already been reached.
// The operation is race-safe: the PostgreSQL upsert's WHERE clause prevents concurrent
// requests from exceeding the limit even under high concurrency.
func (r *TransferRepository) AttemptFileDownload(ctx context.Context, transferID, fileID string, maxDownloads int) (bool, error) {
	var newCount int
	err := r.db.QueryRow(ctx, `
		INSERT INTO transfer.file_download_counts (transfer_id, file_id, count, last_downloaded_at)
		VALUES ($1::uuid, $2::uuid, 1, now())
		ON CONFLICT (transfer_id, file_id) DO UPDATE
		    SET count              = file_download_counts.count + 1,
		        last_downloaded_at = now()
		    WHERE file_download_counts.count < $3
		RETURNING count
	`, transferID, fileID, maxDownloads).Scan(&newCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// WHERE clause was not satisfied — file already at the download limit.
			return false, nil
		}
		return false, apperrors.Internal("attempt file download", err)
	}
	return true, nil
}

// GetFileDownloadCounts returns a map of fileID → download count for all files
// in a transfer that have been downloaded at least once.
func (r *TransferRepository) GetFileDownloadCounts(ctx context.Context, transferID string) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT file_id::text, count FROM transfer.file_download_counts WHERE transfer_id = $1
	`, transferID)
	if err != nil {
		return nil, apperrors.Internal("get file download counts", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var fid string
		var count int
		if err := rows.Scan(&fid, &count); err != nil {
			return nil, apperrors.Internal("scan file download count", err)
		}
		counts[fid] = count
	}
	return counts, rows.Err()
}

// ExpireStale marks all non-revoked transfers whose expires_at is in the past as revoked.
// This is called periodically by a background goroutine to keep the DB consistent.
func (r *TransferRepository) ExpireStale(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE transfer.transfers
		SET is_revoked = TRUE
		WHERE is_revoked = FALSE AND expires_at IS NOT NULL AND expires_at < NOW()
	`)
	if err != nil {
		return 0, apperrors.Internal("expire stale transfers", err)
	}
	return tag.RowsAffected(), nil
}

// GetTransfersNeedingReminder returns active transfers that expire within the next 24 hours
// and have not yet had a reminder email sent (reminder_sent_at IS NULL).
func (r *TransferRepository) GetTransfersNeedingReminder(ctx context.Context) ([]*domain.Transfer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_id, sender_email, recipient_email, slug, name, expires_at
		FROM transfer.transfers
		WHERE is_revoked = FALSE
		  AND reminder_sent_at IS NULL
		  AND expires_at IS NOT NULL
		  AND expires_at > NOW()
		  AND expires_at <= NOW() + INTERVAL '24 hours'
	`)
	if err != nil {
		return nil, apperrors.Internal("get transfers needing reminder", err)
	}
	defer rows.Close()

	var transfers []*domain.Transfer
	for rows.Next() {
		var t domain.Transfer
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.SenderEmail, &t.RecipientEmail, &t.Slug, &t.Name, &t.ExpiresAt); err != nil {
			return nil, apperrors.Internal("scan transfer for reminder", err)
		}
		transfers = append(transfers, &t)
	}
	return transfers, rows.Err()
}

// MarkReminderSent sets reminder_sent_at = NOW() for the given transfer.
func (r *TransferRepository) MarkReminderSent(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE transfer.transfers SET reminder_sent_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		return apperrors.Internal("mark reminder sent", err)
	}
	return nil
}

// UpdateRecipientEmail replaces the recipient_email field (comma-separated) for a transfer.
// Only the owner may do this.
func (r *TransferRepository) UpdateRecipientEmail(ctx context.Context, id, ownerID, recipientEmail string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE transfer.transfers SET recipient_email = $1 WHERE id = $2 AND owner_id = $3
	`, recipientEmail, id, ownerID)
	if err != nil {
		return apperrors.Internal("update recipient_email", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("transfer not found")
	}
	return nil
}

// StoreRecipientToken upserts a recipient magic-link token for a (transfer, email) pair.
// If a token already exists for this pair it is replaced (one active token per recipient).
func (r *TransferRepository) StoreRecipientToken(ctx context.Context, tok *domain.RecipientToken) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO transfer.recipient_tokens (transfer_id, email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (transfer_id, email) DO UPDATE
			SET token_hash = EXCLUDED.token_hash,
			    expires_at = EXCLUDED.expires_at,
			    created_at = now()
	`, tok.TransferID, tok.Email, tok.TokenHash, tok.ExpiresAt)
	if err != nil {
		return apperrors.Internal("store recipient token", err)
	}
	return nil
}

// GetRecipientTokenByHash looks up a recipient token by its SHA-256 hash.
// Returns NotFound if the hash does not match any stored token.
func (r *TransferRepository) GetRecipientTokenByHash(ctx context.Context, tokenHash string) (*domain.RecipientToken, error) {
	tok := &domain.RecipientToken{}
	err := r.db.QueryRow(ctx, `
		SELECT id, transfer_id, email, token_hash, expires_at, created_at
		FROM transfer.recipient_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&tok.ID, &tok.TransferID, &tok.Email, &tok.TokenHash, &tok.ExpiresAt, &tok.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("recipient token not found")
		}
		return nil, apperrors.Internal("get recipient token", err)
	}
	return tok, nil
}

// DeleteRecipientToken removes all tokens for a (transfer, email) pair.
func (r *TransferRepository) DeleteRecipientToken(ctx context.Context, transferID, email string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM transfer.recipient_tokens WHERE transfer_id = $1 AND email = $2
	`, transferID, email)
	if err != nil {
		return apperrors.Internal("delete recipient token", err)
	}
	return nil
}
