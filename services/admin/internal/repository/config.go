package repository

import (
	"context"
	"fmt"

	"github.com/tenzoshare/tenzoshare/services/admin/internal/models"
	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

// ── Config repository (audit, auth, branding, platform, SMTP) ────────────────

// GetAuditConfig returns the audit retention configuration.
func (r *Repository) GetAuditConfig(ctx context.Context) (*models.AuditConfig, error) {
	var cfg models.AuditConfig
	err := r.db.QueryRow(ctx, `
		SELECT retention_enabled, retention_days, updated_at, updated_by
		FROM admin.audit_config
		WHERE id = 1
	`).Scan(&cfg.RetentionEnabled, &cfg.RetentionDays, &cfg.UpdatedAt, &cfg.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("get audit config: %w", err)
	}
	return &cfg, nil
}

// UpdateAuditConfig updates the audit retention configuration.
func (r *Repository) UpdateAuditConfig(ctx context.Context, retentionEnabled *bool, retentionDays *int, adminID string) error {
	updates := []string{}
	args := []any{}
	idx := 1

	if retentionEnabled != nil {
		updates = append(updates, fmt.Sprintf("retention_enabled = $%d", idx))
		args = append(args, *retentionEnabled)
		idx++
	}
	if retentionDays != nil {
		updates = append(updates, fmt.Sprintf("retention_days = $%d", idx))
		args = append(args, *retentionDays)
		idx++
	}

	if len(updates) == 0 {
		return nil
	}

	updates = append(updates, fmt.Sprintf("updated_by = $%d", idx))
	args = append(args, adminID)
	idx++

	sql := fmt.Sprintf("UPDATE admin.audit_config SET %s WHERE id = 1", joinStr(updates, ", "))
	_, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update audit config: %w", err)
	}
	return nil
}

// GetAuditStats returns audit log statistics.
func (r *Repository) GetAuditStats(ctx context.Context) (*models.AuditStats, error) {
	var stats models.AuditStats

	// Scalar counts
	err := r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE timestamp >= NOW() - INTERVAL '24 hours') AS last_24h,
			COUNT(*) FILTER (WHERE timestamp >= NOW() - INTERVAL '7 days') AS last_7d,
			MIN(timestamp::date) AS oldest_date
		FROM audit.audit_logs
	`).Scan(&stats.TotalLogs, &stats.Last24h, &stats.Last7d, &stats.OldestLogDate)
	if err != nil {
		return nil, fmt.Errorf("get audit stats: %w", err)
	}

	// By action breakdown
	rows, err := r.db.Query(ctx, `
		SELECT action, COUNT(*)
		FROM audit.audit_logs
		GROUP BY action
		ORDER BY COUNT(*) DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, fmt.Errorf("query action breakdown: %w", err)
	}
	defer rows.Close()

	stats.ByAction = make(map[string]int64)
	for rows.Next() {
		var action string
		var count int64
		if err := rows.Scan(&action, &count); err != nil {
			return nil, fmt.Errorf("scan action: %w", err)
		}
		stats.ByAction[action] = count
	}
	rows.Close()

	// Eligible for purge count
	cfg, err := r.GetAuditConfig(ctx)
	if err == nil && cfg.RetentionEnabled {
		err = r.db.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM audit.audit_logs
			WHERE timestamp < NOW() - INTERVAL '%d days'
		`, cfg.RetentionDays)).Scan(&stats.EligiblePurge)
		if err != nil {
			return nil, fmt.Errorf("count eligible purge: %w", err)
		}
	}

	return &stats, nil
}

// PurgeAuditLogs deletes audit logs older than the retention period.
func (r *Repository) PurgeAuditLogs(ctx context.Context, retentionDays int) (int64, error) {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM audit.audit_logs
		WHERE timestamp < NOW() - INTERVAL '%d days'
	`, retentionDays))
	if err != nil {
		return 0, fmt.Errorf("purge audit logs: %w", err)
	}
	return result.RowsAffected(), nil
}

// ── Auth config ──────────────────────────────────────────────────────────────

// GetAuthConfig returns the auth/lockout configuration.
func (r *Repository) GetAuthConfig(ctx context.Context) (*models.AuthLockoutConfig, error) {
	var cfg models.AuthLockoutConfig
	err := r.db.QueryRow(ctx, `
		SELECT max_failed_attempts, lockout_duration_minutes, require_mfa,
		       require_email_verification, registration_enabled, updated_at
		FROM auth.auth_settings
		WHERE id = 1
	`).Scan(&cfg.MaxFailedAttempts, &cfg.LockoutDurationMinutes, &cfg.RequireMFA,
		&cfg.RequireEmailVerification, &cfg.RegistrationEnabled, &cfg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get auth config: %w", err)
	}
	return &cfg, nil
}

// UpdateAuthConfig updates the auth configuration.
func (r *Repository) UpdateAuthConfig(ctx context.Context, updates map[string]any) error {
	setClauses := []string{}
	args := []any{}
	idx := 1

	for key, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, idx))
		args = append(args, val)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	sql := fmt.Sprintf("UPDATE auth.auth_settings SET %s WHERE id = 1", joinStr(setClauses, ", "))
	_, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update auth config: %w", err)
	}
	return nil
}

// ── Branding config ──────────────────────────────────────────────────────────

// GetBrandingConfig returns the branding configuration.
func (r *Repository) GetBrandingConfig(ctx context.Context) (*models.BrandingConfig, error) {
	var cfg models.BrandingConfig
	err := r.db.QueryRow(ctx, `
		SELECT primary_color, secondary_color, page_bg_color, surface_color, text_color,
		       border_radius, app_name, custom_css, logo_data_url, updated_at,
		       dm_primary_color, dm_secondary_color, dm_page_bg_color, dm_surface_color, dm_text_color,
		       email_sender_name, email_support_email, email_footer_text, email_subject_prefix,
		       email_header_link, email_reply_to, email_button_color, email_button_text_color,
		       email_body_bg_color, email_card_bg_color, email_card_border_color, email_heading_color, email_text_color,
		       subject_transfer_received, subject_password_reset, subject_email_verification,
		       subject_download_notification, subject_expiry_reminder, subject_transfer_revoked, subject_request_submission,
		       cta_transfer_received, cta_download_notification, cta_password_reset, cta_email_verification,
		       cta_expiry_reminder, cta_request_submission,
		       custom_transfer_received, custom_password_reset, custom_email_verification,
		       custom_download_notification, custom_expiry_reminder, custom_transfer_revoked, custom_request_submission
		FROM admin.branding
		WHERE id = 1
	`).Scan(&cfg.PrimaryColor, &cfg.SecondaryColor, &cfg.PageBgColor, &cfg.SurfaceColor, &cfg.TextColor,
		&cfg.BorderRadius, &cfg.AppName, &cfg.CustomCSS, &cfg.LogoDataURL, &cfg.UpdatedAt,
		&cfg.DmPrimaryColor, &cfg.DmSecondaryColor, &cfg.DmPageBgColor, &cfg.DmSurfaceColor, &cfg.DmTextColor,
		&cfg.EmailSenderName, &cfg.EmailSupportEmail, &cfg.EmailFooterText, &cfg.EmailSubjectPrefix,
		&cfg.EmailHeaderLink, &cfg.EmailReplyTo, &cfg.EmailButtonColor, &cfg.EmailButtonTextColor,
		&cfg.EmailBodyBgColor, &cfg.EmailCardBgColor, &cfg.EmailCardBorderColor, &cfg.EmailHeadingColor, &cfg.EmailTextColor,
		&cfg.SubjectTransferReceived, &cfg.SubjectPasswordReset, &cfg.SubjectEmailVerification,
		&cfg.SubjectDownloadNotification, &cfg.SubjectExpiryReminder, &cfg.SubjectTransferRevoked, &cfg.SubjectRequestSubmission,
		&cfg.CTATransferReceived, &cfg.CTADownloadNotification, &cfg.CTAPasswordReset, &cfg.CTAEmailVerification,
		&cfg.CTAExpiryReminder, &cfg.CTARequestSubmission,
		&cfg.CustomTransferReceived, &cfg.CustomPasswordReset, &cfg.CustomEmailVerification,
		&cfg.CustomDownloadNotification, &cfg.CustomExpiryReminder, &cfg.CustomTransferRevoked, &cfg.CustomRequestSubmission)
	if err != nil {
		return nil, fmt.Errorf("get branding config: %w", err)
	}
	return &cfg, nil
}

// UpdateBrandingConfig updates the branding configuration.
func (r *Repository) UpdateBrandingConfig(ctx context.Context, updates map[string]any) error {
	setClauses := []string{}
	args := []any{}
	idx := 1

	for key, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, idx))
		args = append(args, val)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	sql := fmt.Sprintf("UPDATE admin.branding SET %s WHERE id = 1", joinStr(setClauses, ", "))
	_, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update branding config: %w", err)
	}
	return nil
}

// ── Platform config ──────────────────────────────────────────────────────────

// GetPlatformConfig returns the platform configuration.
func (r *Repository) GetPlatformConfig(ctx context.Context) (*models.PlatformConfig, error) {
	var cfg models.PlatformConfig
	err := r.db.QueryRow(ctx, `
		SELECT date_format, time_format, timezone, portal_url, download_url,
		       link_protection_policy, updated_at
		FROM admin.platform_config
		WHERE id = 1
	`).Scan(&cfg.DateFormat, &cfg.TimeFormat, &cfg.Timezone, &cfg.PortalURL,
		&cfg.DownloadURL, &cfg.LinkProtectionPolicy, &cfg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get platform config: %w", err)
	}
	return &cfg, nil
}

// UpdatePlatformConfig updates the platform configuration.
func (r *Repository) UpdatePlatformConfig(ctx context.Context, updates map[string]any) error {
	setClauses := []string{}
	args := []any{}
	idx := 1

	for key, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, idx))
		args = append(args, val)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	sql := fmt.Sprintf("UPDATE admin.platform_config SET %s WHERE id = 1", joinStr(setClauses, ", "))
	_, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update platform config: %w", err)
	}
	return nil
}

// ── SMTP settings ────────────────────────────────────────────────────────────

// GetSMTPSettings returns the SMTP configuration with decrypted password.
func (r *Repository) GetSMTPSettings(ctx context.Context, encKey []byte) (config.SMTPConfig, error) {
	var cfg config.SMTPConfig
	var encPassword *[]byte

	err := r.db.QueryRow(ctx, `
		SELECT host, port, username, encrypted_password, from_addr, use_tls
		FROM admin.smtp_settings
		WHERE id = 1
	`).Scan(&cfg.Host, &cfg.Port, &cfg.Username, &encPassword, &cfg.From, &cfg.UseTLS)

	if err != nil {
		return cfg, fmt.Errorf("get smtp settings: %w", err)
	}

	// Decrypt password if present
	if encPassword != nil && len(*encPassword) > 0 {
		plainPassword, err := decryptAES(*encPassword, encKey)
		if err != nil {
			return cfg, fmt.Errorf("decrypt password: %w", err)
		}
		cfg.Password = string(plainPassword)
	}

	return cfg, nil
}

// UpdateSMTPSettings updates the SMTP configuration with encrypted password.
func (r *Repository) UpdateSMTPSettings(ctx context.Context, updates map[string]any) error {
	setClauses := []string{}
	args := []any{}
	idx := 1

	for key, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, idx))
		args = append(args, val)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	sql := fmt.Sprintf("UPDATE admin.smtp_settings SET %s WHERE id = 1", joinStr(setClauses, ", "))
	_, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update smtp settings: %w", err)
	}
	return nil
}

// IsSMTPPasswordSet checks if an encrypted password is stored.
func (r *Repository) IsSMTPPasswordSet(ctx context.Context) (bool, error) {
	var encPassword *[]byte
	err := r.db.QueryRow(ctx, `
		SELECT encrypted_password FROM admin.smtp_settings WHERE id = 1
	`).Scan(&encPassword)
	if err != nil {
		return false, fmt.Errorf("check smtp password: %w", err)
	}
	return encPassword != nil && len(*encPassword) > 0, nil
}

// ── Stats ────────────────────────────────────────────────────────────────────

// GetSystemStats returns aggregated system statistics.
func (r *Repository) GetSystemStats(ctx context.Context) (*models.SystemStats, error) {
	var stats models.SystemStats

	// Scalar totals
	err := r.db.QueryRow(ctx, `
		SELECT 
			(SELECT COUNT(*) FROM auth.users) AS total_users,
			(SELECT COUNT(*) FROM auth.users WHERE created_at >= NOW() - INTERVAL '30 days') AS new_users_30d,
			(SELECT COUNT(*) FROM transfer.transfers) AS total_transfers,
			(SELECT COUNT(*) FROM storage.files WHERE deleted_at IS NULL) AS total_files,
			(SELECT COALESCE(SUM(size_bytes), 0) FROM storage.files WHERE deleted_at IS NULL) AS total_storage_bytes
	`).Scan(&stats.TotalUsers, &stats.NewUsers30d, &stats.TotalTransfers, &stats.TotalFiles, &stats.TotalStorageB)
	if err != nil {
		return nil, fmt.Errorf("get system stats: %w", err)
	}

	// Transfer breakdown
	breakdown, err := r.GetTransferBreakdown(ctx)
	if err != nil {
		return nil, err
	}
	stats.TransferBreakdown = *breakdown

	// Transfers per day (last 14 days)
	rows, err := r.db.Query(ctx, `
		SELECT created_at::date AS day, COUNT(*)::int
		FROM transfer.transfers
		WHERE created_at >= CURRENT_DATE - INTERVAL '14 days'
		GROUP BY day
		ORDER BY day
	`)
	if err != nil {
		return nil, fmt.Errorf("query transfers per day: %w", err)
	}
	defer rows.Close()

	stats.TransfersPerDay = make([]models.DayStat, 0)
	for rows.Next() {
		var stat models.DayStat
		if err := rows.Scan(&stat.Day, &stat.Count); err != nil {
			return nil, fmt.Errorf("scan transfer day: %w", err)
		}
		stats.TransfersPerDay = append(stats.TransfersPerDay, stat)
	}
	rows.Close()

	// New users per day (last 14 days)
	rows, err = r.db.Query(ctx, `
		SELECT created_at::date AS day, COUNT(*)::int
		FROM auth.users
		WHERE created_at >= CURRENT_DATE - INTERVAL '14 days'
		GROUP BY day
		ORDER BY day
	`)
	if err != nil {
		return nil, fmt.Errorf("query users per day: %w", err)
	}
	defer rows.Close()

	stats.UsersPerDay = make([]models.DayStat, 0)
	for rows.Next() {
		var stat models.DayStat
		if err := rows.Scan(&stat.Day, &stat.Count); err != nil {
			return nil, fmt.Errorf("scan user day: %w", err)
		}
		stats.UsersPerDay = append(stats.UsersPerDay, stat)
	}
	rows.Close()

	// Storage added per day (last 14 days)
	rows, err = r.db.Query(ctx, `
		SELECT created_at::date AS day, COALESCE(SUM(size_bytes), 0)
		FROM storage.files
		WHERE created_at >= CURRENT_DATE - INTERVAL '14 days'
		GROUP BY day
		ORDER BY day
	`)
	if err != nil {
		return nil, fmt.Errorf("query storage per day: %w", err)
	}
	defer rows.Close()

	stats.StoragePerDay = make([]models.StorageDayStat, 0)
	for rows.Next() {
		var stat models.StorageDayStat
		if err := rows.Scan(&stat.Day, &stat.Bytes); err != nil {
			return nil, fmt.Errorf("scan storage day: %w", err)
		}
		stats.StoragePerDay = append(stats.StoragePerDay, stat)
	}

	return &stats, rows.Err()
}

// ── Crypto helpers ───────────────────────────────────────────────────────────

// decryptAES decrypts ciphertext using AES-256-GCM.
// This is a simplified version - in production, use the shared crypto package.
func decryptAES(ciphertext, key []byte) ([]byte, error) {
	// NOTE: This should use the shared/pkg/crypto package in production.
	// For now, returning a stub implementation.
	return ciphertext, nil // TODO: implement proper decryption
}
