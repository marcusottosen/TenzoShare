package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tenzoshare/tenzoshare/services/storage/internal/domain"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

type FileRepository struct {
	db *pgxpool.Pool

	// Cached storage config (refreshed every 30 seconds to avoid per-upload DB query)
	cfgMu      sync.RWMutex
	cachedCfg  *domain.StorageConfig
	cfgFetchAt time.Time
}

func NewFileRepository(db *pgxpool.Pool) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(ctx context.Context, ownerID, objectKey, filename, contentType string, sizeBytes int64, encryptionIV []byte) (*domain.File, error) {
	var f domain.File
	err := r.db.QueryRow(ctx, `
		INSERT INTO storage.files (owner_id, object_key, filename, content_type, size_bytes, is_encrypted, encryption_iv)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, owner_id, object_key, filename, content_type, size_bytes, is_encrypted, encryption_iv, created_at, deleted_at
	`, ownerID, objectKey, filename, contentType, sizeBytes, encryptionIV != nil, encryptionIV).Scan(
		&f.ID, &f.OwnerID, &f.ObjectKey, &f.Filename,
		&f.ContentType, &f.SizeBytes, &f.IsEncrypted, &f.EncryptionIV, &f.CreatedAt, &f.DeletedAt,
	)
	if err != nil {
		return nil, apperrors.Internal("create file record", err)
	}
	return &f, nil
}

func (r *FileRepository) GetByID(ctx context.Context, id string) (*domain.File, error) {
	var f domain.File
	err := r.db.QueryRow(ctx, `
		SELECT id, owner_id, object_key, filename, content_type, size_bytes, is_encrypted, encryption_iv, created_at, deleted_at
		FROM storage.files
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&f.ID, &f.OwnerID, &f.ObjectKey, &f.Filename,
		&f.ContentType, &f.SizeBytes, &f.IsEncrypted, &f.EncryptionIV, &f.CreatedAt, &f.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("file not found")
		}
		return nil, apperrors.Internal("get file", err)
	}
	return &f, nil
}

func (r *FileRepository) ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*domain.File, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_id, object_key, filename, content_type, size_bytes, is_encrypted, encryption_iv, created_at, deleted_at
		FROM storage.files
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, ownerID, limit, offset)
	if err != nil {
		return nil, apperrors.Internal("list files", err)
	}
	defer rows.Close()

	var files []*domain.File
	for rows.Next() {
		var f domain.File
		if err := rows.Scan(
			&f.ID, &f.OwnerID, &f.ObjectKey, &f.Filename,
			&f.ContentType, &f.SizeBytes, &f.IsEncrypted, &f.EncryptionIV, &f.CreatedAt, &f.DeletedAt,
		); err != nil {
			return nil, apperrors.Internal("scan file row", err)
		}
		files = append(files, &f)
	}
	return files, rows.Err()
}

// ListByOwnerWithShareInfo returns files for a user with live share/expiry context.
// It joins to transfer.transfer_files / transfer.transfers so the caller can show
// when each file will be auto-deleted (if retention is enabled).
func (r *FileRepository) ListByOwnerWithShareInfo(ctx context.Context, ownerID string, limit, offset int) ([]*domain.FileWithShareInfo, error) {
	rows, err := r.db.Query(ctx, `
		WITH file_shares AS (
		    SELECT tf.file_id,
		           count(*)                                                                                  AS share_count,
		           count(*) FILTER (WHERE NOT t.is_revoked AND (t.expires_at IS NULL OR t.expires_at > now())) AS active_shares,
		           max(t.expires_at)                                                                         AS last_exp
		    FROM transfer.transfer_files tf
		    JOIN transfer.transfers t ON t.id = tf.transfer_id
		    GROUP BY tf.file_id
		)
		SELECT f.id, f.owner_id, f.object_key, f.filename, f.content_type, f.size_bytes,
		       f.is_encrypted, f.encryption_iv, f.created_at, f.deleted_at,
		       coalesce(fs.share_count, 0)   AS share_count,
		       coalesce(fs.active_shares, 0) AS active_shares,
		       fs.last_exp
		FROM storage.files f
		LEFT JOIN file_shares fs ON fs.file_id::uuid = f.id
		WHERE f.owner_id = $1 AND f.deleted_at IS NULL
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`, ownerID, limit, offset)
	if err != nil {
		return nil, apperrors.Internal("list files with share info", err)
	}
	defer rows.Close()

	var files []*domain.FileWithShareInfo
	for rows.Next() {
		var fw domain.FileWithShareInfo
		if err := rows.Scan(
			&fw.ID, &fw.OwnerID, &fw.ObjectKey, &fw.Filename, &fw.ContentType, &fw.SizeBytes,
			&fw.IsEncrypted, &fw.EncryptionIV, &fw.CreatedAt, &fw.DeletedAt,
			&fw.ShareCount, &fw.ActiveShares, &fw.LastShareExpiresAt,
		); err != nil {
			return nil, apperrors.Internal("scan file share info row", err)
		}
		files = append(files, &fw)
	}
	return files, rows.Err()
}

// SoftDelete marks a file as deleted (owner-checked — used by the user's own delete action).
func (r *FileRepository) SoftDelete(ctx context.Context, id, ownerID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE storage.files SET deleted_at = $1
		WHERE id = $2 AND owner_id = $3 AND deleted_at IS NULL
	`, time.Now(), id, ownerID)
	if err != nil {
		return apperrors.Internal("delete file record", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("file not found")
	}
	return nil
}

// GetUsageByOwner returns aggregated file count and total bytes for a single owner.
// Only non-deleted files are counted.
func (r *FileRepository) GetUsageByOwner(ctx context.Context, ownerID string) (*domain.UserStorageUsage, error) {
	u := &domain.UserStorageUsage{UserID: ownerID}
	err := r.db.QueryRow(ctx, `
		SELECT count(*), coalesce(sum(size_bytes), 0)
		FROM storage.files
		WHERE owner_id = $1 AND deleted_at IS NULL
	`, ownerID).Scan(&u.FileCount, &u.TotalBytes)
	if err != nil {
		return nil, apperrors.Internal("get storage usage", err)
	}
	return u, nil
}

// GetStorageConfig returns the singleton storage policy.
// Results are cached for 30 seconds to avoid a DB hit on every upload.
func (r *FileRepository) GetStorageConfig(ctx context.Context) (*domain.StorageConfig, error) {
	const cacheTTL = 30 * time.Second

	r.cfgMu.RLock()
	if r.cachedCfg != nil && time.Since(r.cfgFetchAt) < cacheTTL {
		cfg := *r.cachedCfg
		r.cfgMu.RUnlock()
		return &cfg, nil
	}
	r.cfgMu.RUnlock()

	r.cfgMu.Lock()
	defer r.cfgMu.Unlock()
	// Double-check after acquiring write lock
	if r.cachedCfg != nil && time.Since(r.cfgFetchAt) < cacheTTL {
		cfg := *r.cachedCfg
		return &cfg, nil
	}

	var cfg domain.StorageConfig
	err := r.db.QueryRow(ctx, `
		SELECT quota_enabled, quota_bytes_per_user, max_upload_size_bytes,
		       retention_enabled, retention_days, orphan_retention_days,
		       test_mode
		FROM storage.storage_settings WHERE id = 1`,
	).Scan(&cfg.QuotaEnabled, &cfg.QuotaBytesPerUser, &cfg.MaxUploadSizeBytes,
		&cfg.RetentionEnabled, &cfg.RetentionDays, &cfg.OrphanRetentionDays,
		&cfg.TestMode)
	if err != nil {
		return nil, apperrors.Internal("get storage config", err)
	}

	r.cachedCfg = &cfg
	r.cfgFetchAt = time.Now()
	return &cfg, nil
}

// InvalidateConfigCache forces the next GetStorageConfig call to hit the DB.
func (r *FileRepository) InvalidateConfigCache() {
	r.cfgMu.Lock()
	r.cachedCfg = nil
	r.cfgMu.Unlock()
}

// FindFilesEligibleForDeletion returns files that should be deleted by the retention worker.
//
// Two rules:
//  1. retention_expired: file has at least one share; all shares are expired/revoked; the most
//     recent share expiry is older than retentionDays ago.
//  2. orphan_expired: file has NO shares at all and was created more than orphanDays ago.
//
// Only non-deleted files are returned.
func (r *FileRepository) FindFilesEligibleForDeletion(ctx context.Context, retentionDays, orphanDays int) ([]*domain.FileToDelete, error) {
	rows, err := r.db.Query(ctx, `
		WITH shared_files AS (
		    -- files that have at least one transfer
		    SELECT DISTINCT tf.file_id
		    FROM transfer.transfer_files tf
		),
		last_share_expiry AS (
		    -- for each file that was shared, the latest "effective expiry"
		    -- a non-expired / non-revoked share keeps the file alive
		    SELECT tf.file_id,
		           bool_and(t.is_revoked OR (t.expires_at IS NOT NULL AND t.expires_at < now())) AS all_done,
		           max(COALESCE(t.expires_at, now() - interval '1 second')) AS latest_expiry
		    FROM transfer.transfer_files tf
		    JOIN transfer.transfers t ON t.id = tf.transfer_id
		    GROUP BY tf.file_id
		)
		-- Rule 1: file was shared, all shares are finished, and last expiry > retentionDays ago
		SELECT f.id, f.object_key, f.owner_id, f.filename, f.size_bytes, 'retention_expired' AS reason
		FROM storage.files f
		JOIN last_share_expiry lse ON lse.file_id::uuid = f.id
		WHERE f.deleted_at IS NULL
		  AND lse.all_done = true
		  AND lse.latest_expiry < now() - ($1 || ' days')::interval

		UNION ALL

		-- Rule 2: file was never shared, created more than orphanDays ago
		SELECT f.id, f.object_key, f.owner_id, f.filename, f.size_bytes, 'orphan_expired' AS reason
		FROM storage.files f
		WHERE f.deleted_at IS NULL
		  AND f.id NOT IN (SELECT file_id FROM shared_files)
		  AND f.created_at < now() - ($2 || ' days')::interval
	`, retentionDays, orphanDays)
	if err != nil {
		return nil, apperrors.Internal("find files for deletion", err)
	}
	defer rows.Close()

	var out []*domain.FileToDelete
	for rows.Next() {
		var fd domain.FileToDelete
		if err := rows.Scan(&fd.ID, &fd.ObjectKey, &fd.OwnerID, &fd.Filename, &fd.SizeBytes, &fd.Reason); err != nil {
			return nil, apperrors.Internal("scan file for deletion", err)
		}
		out = append(out, &fd)
	}
	return out, rows.Err()
}

// SoftDeleteByID marks a file as deleted (without owner check — used by the cleanup worker).
func (r *FileRepository) SoftDeleteByID(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE storage.files SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return apperrors.Internal("soft delete file by id", err)
	}
	return nil
}

// FindSoftDeletedPendingObjectPurge returns recently soft-deleted files (last 48 h)
// whose MinIO objects may not have been deleted yet (e.g. admin-deleted files).
// The caller should attempt a best-effort MinIO delete for each.
func (r *FileRepository) FindSoftDeletedPendingObjectPurge(ctx context.Context) ([]*domain.FileToDelete, error) {
	rows, err := r.db.Query(ctx, `
		SELECT f.id, f.object_key, f.owner_id, f.filename, f.size_bytes
		FROM storage.files f
		WHERE f.deleted_at IS NOT NULL
		  AND f.deleted_at > now() - interval '48 hours'
		  AND f.id NOT IN (SELECT file_id FROM storage.file_purge_log)
		ORDER BY f.deleted_at DESC
		LIMIT 200
	`)
	if err != nil {
		return nil, apperrors.Internal("find soft-deleted files pending object purge", err)
	}
	defer rows.Close()

	var out []*domain.FileToDelete
	for rows.Next() {
		fd := &domain.FileToDelete{Reason: "admin_purge"}
		if err := rows.Scan(&fd.ID, &fd.ObjectKey, &fd.OwnerID, &fd.Filename, &fd.SizeBytes); err != nil {
			return nil, apperrors.Internal("scan pending object purge row", err)
		}
		out = append(out, fd)
	}
	return out, rows.Err()
}

// RecordPurge writes an entry to the file_purge_log audit table.
func (r *FileRepository) RecordPurge(ctx context.Context, fd *domain.FileToDelete, purgedBy string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO storage.file_purge_log (file_id, owner_id, filename, size_bytes, reason, purged_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, fd.ID, fd.OwnerID, fd.Filename, fd.SizeBytes, fd.Reason, purgedBy)
	if err != nil {
		return apperrors.Internal("record purge log", err)
	}
	return nil
}
