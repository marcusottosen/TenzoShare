package repository

import (
	"context"
	"fmt"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/models"
)

// ── Storage repository ───────────────────────────────────────────────────────

// ListStorageUsage returns per-user storage stats with pagination.
func (r *Repository) ListStorageUsage(ctx context.Context, limit, offset int, sortBy, sortDir string) ([]models.StorageUserUsage, int, error) {
	// Count total users
	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM auth.users").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// Build query with aggregation
	orderBy := storageUsageSortClause(sortBy, sortDir)
	sql := `
		SELECT 
			u.id AS user_id,
			u.email,
			COUNT(f.id)::bigint AS file_count,
			COALESCE(SUM(f.size_bytes), 0) AS total_bytes
		FROM auth.users u
		LEFT JOIN storage.files f ON f.owner_id = u.id AND f.deleted_at IS NULL
		GROUP BY u.id, u.email
		ORDER BY ` + orderBy + `
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query storage usage: %w", err)
	}
	defer rows.Close()

	usage := make([]models.StorageUserUsage, 0)
	for rows.Next() {
		var u models.StorageUserUsage
		if err := rows.Scan(&u.UserID, &u.Email, &u.FileCount, &u.TotalBytes); err != nil {
			return nil, 0, fmt.Errorf("scan usage: %w", err)
		}
		usage = append(usage, u)
	}

	return usage, total, rows.Err()
}

// GetStorageConfig returns the singleton storage configuration.
func (r *Repository) GetStorageConfig(ctx context.Context) (*models.StorageConfig, error) {
	var cfg models.StorageConfig
	err := r.db.QueryRow(ctx, `
		SELECT quota_enabled, quota_bytes_per_user, max_upload_size_bytes,
		       retention_enabled, retention_days, orphan_retention_days,
		       test_mode, updated_at, updated_by
		FROM admin.storage_config
		WHERE id = 1
	`).Scan(&cfg.QuotaEnabled, &cfg.QuotaBytesPerUser, &cfg.MaxUploadSizeBytes,
		&cfg.RetentionEnabled, &cfg.RetentionDays, &cfg.OrphanRetentionDays,
		&cfg.TestMode, &cfg.UpdatedAt, &cfg.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("get storage config: %w", err)
	}
	return &cfg, nil
}

// UpdateStorageConfig updates the storage configuration.
func (r *Repository) UpdateStorageConfig(ctx context.Context, updates map[string]any, adminID string) error {
	// Build dynamic UPDATE
	setClauses := []string{}
	args := []any{}
	idx := 1

	for key, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, idx))
		args = append(args, val)
		idx++
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_by = $%d", idx))
	args = append(args, adminID)
	idx++

	sql := fmt.Sprintf("UPDATE admin.storage_config SET %s WHERE id = 1", joinStr(setClauses, ", "))
	_, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update storage config: %w", err)
	}
	return nil
}

type FileFilters struct {
	Filter  string // "all", "orphan", "eligible"
	Limit   int
	Offset  int
	SortBy  string
	SortDir string
}

// ListStorageFiles returns files with share context.
func (r *Repository) ListStorageFiles(ctx context.Context, filters FileFilters) ([]models.AdminFileRow, int, error) {
	cfg, err := r.GetStorageConfig(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Build WHERE clause based on filter
	where := " WHERE f.deleted_at IS NULL"
	if filters.Filter == "orphan" {
		where += " AND shares.share_count = 0"
	} else if filters.Filter == "eligible" && cfg.RetentionEnabled {
		where += fmt.Sprintf(` AND (
			(shares.share_count = 0 AND f.created_at < NOW() - INTERVAL '%d days')
			OR (shares.active_shares = 0 AND shares.last_share_exp_at IS NOT NULL AND shares.last_share_exp_at < NOW() - INTERVAL '%d days')
		)`, cfg.OrphanRetentionDays, cfg.RetentionDays)
	}

	// Count
	countSQL := `
		WITH shares AS (
			SELECT 
				tf.file_id,
				COUNT(t.id)::int AS share_count,
				COUNT(t.id) FILTER (WHERE NOT t.is_revoked AND (t.expires_at IS NULL OR t.expires_at > NOW()))::int AS active_shares,
				MAX(t.expires_at) AS last_share_exp_at
			FROM transfer.transfer_files tf
			JOIN transfer.transfers t ON t.id = tf.transfer_id
			GROUP BY tf.file_id
		)
		SELECT COUNT(*) FROM storage.files f
		LEFT JOIN shares ON shares.file_id = f.id
	` + where

	var total int
	if err := r.db.QueryRow(ctx, countSQL).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count files: %w", err)
	}

	// Data query
	orderBy := storageSortClause(filters.SortBy, filters.SortDir)
	dataSQL := `
		WITH shares AS (
			SELECT 
				tf.file_id,
				COUNT(t.id)::int AS share_count,
				COUNT(t.id) FILTER (WHERE NOT t.is_revoked AND (t.expires_at IS NULL OR t.expires_at > NOW()))::int AS active_shares,
				MAX(t.expires_at) AS last_share_exp_at
			FROM transfer.transfer_files tf
			JOIN transfer.transfers t ON t.id = tf.transfer_id
			GROUP BY tf.file_id
		)
		SELECT 
			f.id, f.owner_id, u.email AS owner_email, f.original_filename, f.content_type,
			f.size_bytes, f.created_at,
			COALESCE(shares.share_count, 0) AS share_count,
			COALESCE(shares.active_shares, 0) AS active_shares,
			shares.last_share_exp_at
		FROM storage.files f
		JOIN auth.users u ON u.id = f.owner_id
		LEFT JOIN shares ON shares.file_id = f.id
	` + where + " ORDER BY " + orderBy + " LIMIT $1 OFFSET $2"

	rows, err := r.db.Query(ctx, dataSQL, filters.Limit, filters.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	files := make([]models.AdminFileRow, 0)
	for rows.Next() {
		var file models.AdminFileRow
		if err := rows.Scan(&file.ID, &file.OwnerID, &file.OwnerEmail, &file.Filename,
			&file.ContentType, &file.SizeBytes, &file.CreatedAt, &file.ShareCount,
			&file.ActiveShares, &file.LastShareExpAt); err != nil {
			return nil, 0, fmt.Errorf("scan file: %w", err)
		}

		// Determine eligibility
		file.EligiblePurge = cfg.RetentionEnabled && ((file.ShareCount == 0) ||
			(file.ActiveShares == 0 && file.LastShareExpAt != nil))

		files = append(files, file)
	}

	return files, total, rows.Err()
}

// GetFile returns a single file's metadata.
func (r *Repository) GetFile(ctx context.Context, fileID string) (*models.AdminFileRow, error) {
	var file models.AdminFileRow
	err := r.db.QueryRow(ctx, `
		SELECT f.id, f.owner_id, u.email AS owner_email, f.original_filename,
		       f.content_type, f.size_bytes, f.created_at
		FROM storage.files f
		JOIN auth.users u ON u.id = f.owner_id
		WHERE f.id = $1
	`, fileID).Scan(&file.ID, &file.OwnerID, &file.OwnerEmail, &file.Filename,
		&file.ContentType, &file.SizeBytes, &file.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	return &file, nil
}

// SoftDeleteFile marks a file as deleted.
func (r *Repository) SoftDeleteFile(ctx context.Context, fileID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE storage.files 
		SET deleted_at = NOW()
		WHERE id = $1
	`, fileID)
	if err != nil {
		return fmt.Errorf("soft delete file: %w", err)
	}
	return nil
}

// LogFilePurge records a file deletion in the audit table.
func (r *Repository) LogFilePurge(ctx context.Context, fileID, ownerID, filename string, sizeBytes int64, reason, deletedBy, deletedByType string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO admin.file_purge_log (file_id, owner_id, filename, size_bytes, reason, deleted_by, deleted_by_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, fileID, ownerID, filename, sizeBytes, reason, deletedBy, deletedByType)
	if err != nil {
		return fmt.Errorf("log file purge: %w", err)
	}
	return nil
}

// GetEligiblePurgeFiles returns files eligible for automated purge.
func (r *Repository) GetEligiblePurgeFiles(ctx context.Context, limit int) ([]models.AdminFileRow, error) {
	cfg, err := r.GetStorageConfig(ctx)
	if err != nil {
		return nil, err
	}

	if !cfg.RetentionEnabled {
		return []models.AdminFileRow{}, nil
	}

	sql := `
		WITH shares AS (
			SELECT 
				tf.file_id,
				COUNT(t.id)::int AS share_count,
				COUNT(t.id) FILTER (WHERE NOT t.is_revoked AND (t.expires_at IS NULL OR t.expires_at > NOW()))::int AS active_shares,
				MAX(t.expires_at) AS last_share_exp_at
			FROM transfer.transfer_files tf
			JOIN transfer.transfers t ON t.id = tf.transfer_id
			GROUP BY tf.file_id
		)
		SELECT 
			f.id, f.owner_id, u.email AS owner_email, f.original_filename,
			f.content_type, f.size_bytes, f.created_at,
			COALESCE(shares.share_count, 0) AS share_count,
			COALESCE(shares.active_shares, 0) AS active_shares,
			shares.last_share_exp_at
		FROM storage.files f
		JOIN auth.users u ON u.id = f.owner_id
		LEFT JOIN shares ON shares.file_id = f.id
		WHERE f.deleted_at IS NULL AND (
			(shares.share_count = 0 AND f.created_at < NOW() - INTERVAL '%d days')
			OR (shares.active_shares = 0 AND shares.last_share_exp_at IS NOT NULL AND shares.last_share_exp_at < NOW() - INTERVAL '%d days')
		)
		ORDER BY f.created_at
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, fmt.Sprintf(sql, cfg.OrphanRetentionDays, cfg.RetentionDays), limit)
	if err != nil {
		return nil, fmt.Errorf("query eligible files: %w", err)
	}
	defer rows.Close()

	files := make([]models.AdminFileRow, 0)
	for rows.Next() {
		var file models.AdminFileRow
		if err := rows.Scan(&file.ID, &file.OwnerID, &file.OwnerEmail, &file.Filename,
			&file.ContentType, &file.SizeBytes, &file.CreatedAt, &file.ShareCount,
			&file.ActiveShares, &file.LastShareExpAt); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		file.EligiblePurge = true
		files = append(files, file)
	}

	return files, rows.Err()
}

// GetStorageInsights returns aggregated storage statistics.
func (r *Repository) GetStorageInsights(ctx context.Context) (*models.StorageInsights, error) {
	var insights models.StorageInsights

	// Scalar totals
	err := r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE deleted_at IS NULL) AS total_files,
			COALESCE(SUM(size_bytes) FILTER (WHERE deleted_at IS NULL), 0) AS total_bytes,
			COUNT(*) FILTER (WHERE deleted_at IS NOT NULL) AS deleted_files,
			COUNT(DISTINCT owner_id) FILTER (WHERE deleted_at IS NULL) AS unique_owners
		FROM storage.files
	`).Scan(&insights.TotalFiles, &insights.TotalStorageBytes, &insights.DeletedFiles, &insights.UniqueOwners)
	if err != nil {
		return nil, fmt.Errorf("get totals: %w", err)
	}

	// Purge stats from log
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM admin.file_purge_log
	`).Scan(&insights.PurgedFiles, &insights.FreedBytes)
	if err != nil {
		return nil, fmt.Errorf("get purge stats: %w", err)
	}

	// Content-type breakdown (top 12)
	rows, err := r.db.Query(ctx, `
		SELECT content_type, COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM storage.files
		WHERE deleted_at IS NULL
		GROUP BY content_type
		ORDER BY SUM(size_bytes) DESC NULLS LAST
		LIMIT 12
	`)
	if err != nil {
		return nil, fmt.Errorf("query content types: %w", err)
	}
	defer rows.Close()

	insights.ContentTypeBreakdown = make([]models.ContentTypeStat, 0)
	for rows.Next() {
		var stat models.ContentTypeStat
		if err := rows.Scan(&stat.ContentType, &stat.Count, &stat.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan content type: %w", err)
		}
		insights.ContentTypeBreakdown = append(insights.ContentTypeBreakdown, stat)
	}
	rows.Close()

	// Purge reason breakdown
	rows, err = r.db.Query(ctx, `
		SELECT reason, COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM admin.file_purge_log
		GROUP BY reason
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query purge reasons: %w", err)
	}
	defer rows.Close()

	insights.PurgeReasonBreakdown = make([]models.PurgeReasonStat, 0)
	for rows.Next() {
		var stat models.PurgeReasonStat
		if err := rows.Scan(&stat.Reason, &stat.Count, &stat.FreedBytes); err != nil {
			return nil, fmt.Errorf("scan purge reason: %w", err)
		}
		insights.PurgeReasonBreakdown = append(insights.PurgeReasonBreakdown, stat)
	}
	rows.Close()

	// Purge per day (last 30 days)
	rows, err = r.db.Query(ctx, `
		SELECT deleted_at::date AS day, COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM admin.file_purge_log
		WHERE deleted_at >= CURRENT_DATE - INTERVAL '30 days'
		GROUP BY day
		ORDER BY day
	`)
	if err != nil {
		return nil, fmt.Errorf("query purge per day: %w", err)
	}
	defer rows.Close()

	insights.PurgePerDay = make([]models.PurgeDayStat, 0)
	for rows.Next() {
		var stat models.PurgeDayStat
		if err := rows.Scan(&stat.Day, &stat.Count, &stat.FreedBytes); err != nil {
			return nil, fmt.Errorf("scan purge day: %w", err)
		}
		insights.PurgePerDay = append(insights.PurgePerDay, stat)
	}
	rows.Close()

	// Storage added per day (last 30 days)
	rows, err = r.db.Query(ctx, `
		SELECT created_at::date AS day, COALESCE(SUM(size_bytes), 0)
		FROM storage.files
		WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
		GROUP BY day
		ORDER BY day
	`)
	if err != nil {
		return nil, fmt.Errorf("query storage per day: %w", err)
	}
	defer rows.Close()

	insights.StoragePerDay = make([]models.StorageDayStat, 0)
	for rows.Next() {
		var stat models.StorageDayStat
		if err := rows.Scan(&stat.Day, &stat.Bytes); err != nil {
			return nil, fmt.Errorf("scan storage day: %w", err)
		}
		insights.StoragePerDay = append(insights.StoragePerDay, stat)
	}

	return &insights, rows.Err()
}

// ListPurgeLog returns paginated purge log entries.
func (r *Repository) ListPurgeLog(ctx context.Context, limit, offset int) ([]models.PurgeLogEntry, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM admin.file_purge_log").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count purge log: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT l.id, l.file_id, l.owner_id, u.email AS owner_email, l.filename,
		       l.size_bytes, l.reason, l.deleted_by, l.deleted_by_type, l.deleted_at
		FROM admin.file_purge_log l
		JOIN auth.users u ON u.id = l.owner_id
		ORDER BY l.deleted_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query purge log: %w", err)
	}
	defer rows.Close()

	entries := make([]models.PurgeLogEntry, 0)
	for rows.Next() {
		var entry models.PurgeLogEntry
		if err := rows.Scan(&entry.ID, &entry.FileID, &entry.OwnerID, &entry.OwnerEmail,
			&entry.Filename, &entry.SizeBytes, &entry.Reason, &entry.DeletedBy,
			&entry.DeletedByType, &entry.DeletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan purge log: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, total, rows.Err()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func storageUsageSortClause(sortBy, sortDir string) string {
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "email":
		return "u.email " + sortDir
	case "file_count":
		return "file_count " + sortDir
	default:
		return "total_bytes " + sortDir
	}
}

func storageSortClause(sortBy, sortDir string) string {
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "filename":
		return "f.original_filename " + sortDir
	case "owner_email":
		return "u.email " + sortDir
	case "created_at":
		return "f.created_at " + sortDir
	default:
		return "f.size_bytes " + sortDir
	}
}
