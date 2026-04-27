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
		SELECT quota_enabled, quota_bytes_per_user, max_upload_size_bytes
		FROM storage.storage_settings WHERE id = 1`,
	).Scan(&cfg.QuotaEnabled, &cfg.QuotaBytesPerUser, &cfg.MaxUploadSizeBytes)
	if err != nil {
		return nil, apperrors.Internal("get storage config", err)
	}

	r.cachedCfg = &cfg
	r.cfgFetchAt = time.Now()
	return &cfg, nil
}
