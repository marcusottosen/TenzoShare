package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/models"
)

// ── Transfer repository ──────────────────────────────────────────────────────

type TransferFilters struct {
	Status  string // "all", "active", "expired", "revoked"
	Limit   int
	Offset  int
	SortBy  string
	SortDir string
}

// ListTransfers returns a paginated list of all transfers with aggregated file info.
func (r *Repository) ListTransfers(ctx context.Context, filters TransferFilters) ([]models.TransferRow, int, error) {
	args := []any{}
	where := " WHERE 1=1"
	idx := 1

	// Status filter
	if filters.Status == "active" {
		where += fmt.Sprintf(" AND t.is_revoked = false AND (t.expires_at IS NULL OR t.expires_at > NOW()) AND exhausted = false")
	} else if filters.Status == "expired" {
		where += fmt.Sprintf(" AND t.is_revoked = false AND t.expires_at IS NOT NULL AND t.expires_at <= NOW()")
	} else if filters.Status == "revoked" {
		where += fmt.Sprintf(" AND t.is_revoked = true")
	}

	// Count total
	countSQL := `
		WITH agg AS (
			SELECT 
				t.id,
				COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) AS total_downloads,
				(t.max_downloads IS NOT NULL AND COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) >= t.max_downloads) AS exhausted
			FROM transfer.transfers t
			LEFT JOIN transfer.transfer_files tf ON tf.transfer_id = t.id
			LEFT JOIN transfer.download_counts dc ON dc.transfer_file_id = tf.id
			GROUP BY t.id
		)
		SELECT COUNT(*) FROM transfer.transfers t
		JOIN agg ON agg.id = t.id
	` + where

	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transfers: %w", err)
	}

	// Build main query
	orderBy := transferSortClause(filters.SortBy, filters.SortDir)

	dataSQL := `
		WITH agg AS (
			SELECT 
				t.id,
				COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) AS total_downloads,
				(t.max_downloads IS NOT NULL AND COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) >= t.max_downloads) AS exhausted
			FROM transfer.transfers t
			LEFT JOIN transfer.transfer_files tf ON tf.transfer_id = t.id
			LEFT JOIN transfer.download_counts dc ON dc.transfer_file_id = tf.id
			GROUP BY t.id
		),
		files AS (
			SELECT 
				tf.transfer_id, 
				COUNT(*)::int AS file_count, 
				COALESCE(SUM(f.size_bytes), 0) AS total_size_bytes
			FROM transfer.transfer_files tf
			JOIN storage.files f ON f.id = tf.file_id
			GROUP BY tf.transfer_id
		)
		SELECT 
			t.id, u.email AS owner_email, t.name, t.description,
			COALESCE(t.recipient_email, '') AS recipient_email,
			t.slug, t.is_revoked, t.expires_at,
			agg.total_downloads AS download_count, t.max_downloads,
			t.view_only, t.created_at,
			CASE 
				WHEN t.is_revoked THEN 'revoked'
				WHEN agg.exhausted THEN 'exhausted'
				WHEN t.expires_at IS NOT NULL AND t.expires_at <= NOW() THEN 'expired'
				ELSE 'active'
			END AS status,
			(t.password_hash IS NOT NULL) AS has_password,
			COALESCE(files.file_count, 0) AS file_count,
			COALESCE(files.total_size_bytes, 0) AS total_size_bytes,
			agg.exhausted
		FROM transfer.transfers t
		JOIN auth.users u ON u.id = t.owner_id
		JOIN agg ON agg.id = t.id
		LEFT JOIN files ON files.transfer_id = t.id
	` + where + " ORDER BY " + orderBy + " LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query transfers: %w", err)
	}
	defer rows.Close()

	transfers := make([]models.TransferRow, 0)
	for rows.Next() {
		var t models.TransferRow
		var expiresAt *time.Time
		var maxDownloads *int
		var createdAt time.Time
		var passwordHash *string

		if err := rows.Scan(&t.ID, &t.OwnerEmail, &t.Name, &t.Description, &t.RecipientEmail,
			&t.Slug, &t.IsRevoked, &expiresAt, &t.DownloadCount, &maxDownloads, &t.ViewOnly,
			&createdAt, &t.Status, &passwordHash, &t.FileCount, &t.TotalSizeBytes, &t.IsExhausted); err != nil {
			return nil, 0, fmt.Errorf("scan transfer: %w", err)
		}

		scanTransferRow(&t, expiresAt, maxDownloads, createdAt, passwordHash)
		transfers = append(transfers, t)
	}

	return transfers, total, rows.Err()
}

// GetTransfer returns a single transfer with its files.
func (r *Repository) GetTransfer(ctx context.Context, transferID string) (*models.TransferDetail, error) {
	// Get transfer metadata
	var detail models.TransferDetail
	var expiresAt *time.Time
	var maxDownloads *int
	var createdAt time.Time
	var passwordHash *string

	err := r.db.QueryRow(ctx, `
		WITH agg AS (
			SELECT 
				t.id,
				COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) AS total_downloads,
				(t.max_downloads IS NOT NULL AND COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) >= t.max_downloads) AS exhausted
			FROM transfer.transfers t
			LEFT JOIN transfer.transfer_files tf ON tf.transfer_id = t.id
			LEFT JOIN transfer.download_counts dc ON dc.transfer_file_id = tf.id
			WHERE t.id = $1
			GROUP BY t.id
		),
		files AS (
			SELECT 
				tf.transfer_id, 
				COUNT(*)::int AS file_count, 
				COALESCE(SUM(f.size_bytes), 0) AS total_size_bytes
			FROM transfer.transfer_files tf
			JOIN storage.files f ON f.id = tf.file_id
			WHERE tf.transfer_id = $1
			GROUP BY tf.transfer_id
		)
		SELECT 
			t.id, u.email AS owner_email, t.name, t.description,
			COALESCE(t.recipient_email, '') AS recipient_email,
			t.slug, t.is_revoked, t.expires_at,
			agg.total_downloads AS download_count, t.max_downloads,
			t.view_only, t.created_at,
			CASE 
				WHEN t.is_revoked THEN 'revoked'
				WHEN agg.exhausted THEN 'exhausted'
				WHEN t.expires_at IS NOT NULL AND t.expires_at <= NOW() THEN 'expired'
				ELSE 'active'
			END AS status,
			t.password_hash,
			COALESCE(files.file_count, 0) AS file_count,
			COALESCE(files.total_size_bytes, 0) AS total_size_bytes,
			agg.exhausted
		FROM transfer.transfers t
		JOIN auth.users u ON u.id = t.owner_id
		JOIN agg ON agg.id = t.id
		LEFT JOIN files ON files.transfer_id = t.id
		WHERE t.id = $1
	`, transferID).Scan(&detail.ID, &detail.OwnerEmail, &detail.Name, &detail.Description,
		&detail.RecipientEmail, &detail.Slug, &detail.IsRevoked, &expiresAt, &detail.DownloadCount,
		&maxDownloads, &detail.ViewOnly, &createdAt, &detail.Status, &passwordHash,
		&detail.FileCount, &detail.TotalSizeBytes, &detail.IsExhausted)

	if err != nil {
		return nil, fmt.Errorf("get transfer: %w", err)
	}

	scanTransferRow(&detail.TransferRow, expiresAt, maxDownloads, createdAt, passwordHash)

	// Get files
	rows, err := r.db.Query(ctx, `
		SELECT f.id, f.original_filename, f.content_type, f.size_bytes
		FROM transfer.transfer_files tf
		JOIN storage.files f ON f.id = tf.file_id
		WHERE tf.transfer_id = $1
		ORDER BY tf.created_at
	`, transferID)
	if err != nil {
		return nil, fmt.Errorf("query transfer files: %w", err)
	}
	defer rows.Close()

	detail.Files = make([]models.TransferFile, 0)
	for rows.Next() {
		var f models.TransferFile
		if err := rows.Scan(&f.FileID, &f.Filename, &f.ContentType, &f.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		detail.Files = append(detail.Files, f)
	}

	return &detail, rows.Err()
}

// RevokeTransfer marks a transfer as revoked.
func (r *Repository) RevokeTransfer(ctx context.Context, transferID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE transfer.transfers 
		SET is_revoked = true, updated_at = $1
		WHERE id = $2
	`, time.Now(), transferID)
	if err != nil {
		return fmt.Errorf("revoke transfer: %w", err)
	}
	return nil
}

// GetTransferBreakdown returns transfer counts by status.
func (r *Repository) GetTransferBreakdown(ctx context.Context) (*models.TransferBreakdown, error) {
	var breakdown models.TransferBreakdown

	// Build aggregation query with download_counts for exhausted check
	err := r.db.QueryRow(ctx, `
		WITH agg AS (
			SELECT 
				t.id,
				COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) AS total_downloads,
				(t.max_downloads IS NOT NULL AND COALESCE(SUM(CASE WHEN dc.transfer_file_id IS NOT NULL THEN dc.count ELSE 0 END), 0) >= t.max_downloads) AS exhausted
			FROM transfer.transfers t
			LEFT JOIN transfer.transfer_files tf ON tf.transfer_id = t.id
			LEFT JOIN transfer.download_counts dc ON dc.transfer_file_id = tf.id
			GROUP BY t.id
		)
		SELECT 
			COUNT(*) FILTER (WHERE NOT t.is_revoked AND (t.expires_at IS NULL OR t.expires_at > NOW()) AND NOT agg.exhausted) AS active,
			COUNT(*) FILTER (WHERE NOT t.is_revoked AND agg.exhausted) AS exhausted,
			COUNT(*) FILTER (WHERE NOT t.is_revoked AND t.expires_at IS NOT NULL AND t.expires_at <= NOW()) AS expired,
			COUNT(*) FILTER (WHERE t.is_revoked) AS revoked
		FROM transfer.transfers t
		JOIN agg ON agg.id = t.id
	`).Scan(&breakdown.Active, &breakdown.Exhausted, &breakdown.Expired, &breakdown.Revoked)

	if err != nil {
		return nil, fmt.Errorf("get transfer breakdown: %w", err)
	}

	return &breakdown, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func transferSortClause(sortBy, sortDir string) string {
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "name":
		return "t.name " + sortDir
	case "owner_email":
		return "u.email " + sortDir
	case "status":
		return "status " + sortDir
	case "expires_at":
		return "t.expires_at " + sortDir + " NULLS LAST"
	case "file_count":
		return "file_count " + sortDir
	case "total_size_bytes":
		return "total_size_bytes " + sortDir
	default:
		return "t.created_at " + sortDir
	}
}

func scanTransferRow(t *models.TransferRow, expiresAt *time.Time, maxDownloads *int, createdAt time.Time, passwordHash *string) {
	if expiresAt != nil {
		exp := expiresAt.Format(time.RFC3339)
		t.ExpiresAt = &exp
	}
	t.MaxDownloads = maxDownloads
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.HasPassword = (passwordHash != nil && *passwordHash != "")
}
