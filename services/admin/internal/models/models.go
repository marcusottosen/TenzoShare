package models
package models

import "time"

// ── User models ──────────────────────────────────────────────────────────────

type UserRow struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	EmailVerified       bool       `json:"email_verified"`
	MFAEnabled          bool       `json:"mfa_enabled"`
	FailedLoginAttempts int        `json:"failed_login_attempts"`
	LockedUntil         *time.Time `json:"locked_until"`
	LastLoginAt         *time.Time `json:"last_login_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type UserQuota struct {
	UserID     string  `json:"user_id"`
	Email      string  `json:"email"`
	QuotaBytes *int64  `json:"quota_bytes"` // nil = use system default
	UpdatedAt  string  `json:"updated_at"`
	UpdatedBy  *string `json:"updated_by"`
}

// ── Transfer models ──────────────────────────────────────────────────────────

type TransferRow struct {
	ID             string  `json:"id"`
	OwnerEmail     string  `json:"owner_email"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	RecipientEmail string  `json:"recipient_email"`
	Slug           string  `json:"slug"`
	IsRevoked      bool    `json:"is_revoked"`
	ExpiresAt      *string `json:"expires_at"`
	DownloadCount  int     `json:"download_count"`
	MaxDownloads   *int    `json:"max_downloads"`
	ViewOnly       bool    `json:"view_only"`
	CreatedAt      string  `json:"created_at"`
	Status         string  `json:"status"`
	HasPassword    bool    `json:"has_password"`
	FileCount      int     `json:"file_count"`
	TotalSizeBytes int64   `json:"total_size_bytes"`
	IsExhausted    bool    `json:"-"` // populated by DB subquery; not exposed directly
}

type TransferFile struct {
	FileID      string `json:"file_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type TransferDetail struct {
	TransferRow
	Files []TransferFile `json:"files"`
}

type TransferBreakdown struct {
	Active    int `json:"active"`
	Exhausted int `json:"exhausted"`
	Expired   int `json:"expired"`
	Revoked   int `json:"revoked"`
}

// ── Storage models ───────────────────────────────────────────────────────────

type StorageUserUsage struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	FileCount  int64  `json:"file_count"`
	TotalBytes int64  `json:"total_bytes"`
}

type StorageConfig struct {
	QuotaEnabled        bool   `json:"quota_enabled"`
	QuotaBytesPerUser   int64  `json:"quota_bytes_per_user"`
	MaxUploadSizeBytes  int64  `json:"max_upload_size_bytes"`
	RetentionEnabled    bool   `json:"retention_enabled"`
	RetentionDays       int    `json:"retention_days"`
	OrphanRetentionDays int    `json:"orphan_retention_days"`
	TestMode            bool   `json:"test_mode"`
	UpdatedAt           string `json:"updated_at"`
	UpdatedBy           string `json:"updated_by"`
}

type AdminFileRow struct {
	ID             string  `json:"id"`
	OwnerID        string  `json:"owner_id"`
	OwnerEmail     string  `json:"owner_email"`
	Filename       string  `json:"filename"`
	ContentType    string  `json:"content_type"`
	SizeBytes      int64   `json:"size_bytes"`
	CreatedAt      string  `json:"created_at"`
	ShareCount     int     `json:"share_count"`
	ActiveShares   int     `json:"active_shares"`
	LastShareExpAt *string `json:"last_share_expires_at"`
	EligiblePurge  bool    `json:"eligible_purge"`
}

type ContentTypeStat struct {
	ContentType string `json:"content_type"`
	Count       int64  `json:"count"`
	SizeBytes   int64  `json:"size_bytes"`
}

type PurgeReasonStat struct {
	Reason     string `json:"reason"`
	Count      int64  `json:"count"`
	FreedBytes int64  `json:"freed_bytes"`
}

type PurgeDayStat struct {
	Day        string `json:"day"`
	Count      int64  `json:"count"`
	FreedBytes int64  `json:"freed_bytes"`
}

type StorageDayStat struct {
	Day   string `json:"day"`
	Bytes int64  `json:"bytes"`
}

type StorageInsights struct {
	TotalFiles           int64             `json:"total_files"`
	TotalStorageBytes    int64             `json:"total_storage_bytes"`
	DeletedFiles         int64             `json:"deleted_files"`
	PurgedFiles          int64             `json:"purged_files"`
	FreedBytes           int64             `json:"freed_bytes"`
	UniqueOwners         int64             `json:"unique_owners"`
	ContentTypeBreakdown []ContentTypeStat `json:"content_type_breakdown"`
	PurgeReasonBreakdown []PurgeReasonStat `json:"purge_reason_breakdown"`
	PurgePerDay          []PurgeDayStat    `json:"purge_per_day"`
	StoragePerDay        []StorageDayStat  `json:"storage_per_day"`
}

type PurgeLogEntry struct {
	ID            string `json:"id"`
	FileID        string `json:"file_id"`
	OwnerID       string `json:"owner_id"`
	OwnerEmail    string `json:"owner_email"`
	Filename      string `json:"filename"`
	SizeBytes     int64  `json:"size_bytes"`
	Reason        string `json:"reason"`
	DeletedBy     string `json:"deleted_by"`
	DeletedByType string `json:"deleted_by_type"`
	DeletedAt     string `json:"deleted_at"`
}

// ── Stats models ─────────────────────────────────────────────────────────────

type DayStat struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type SystemStats struct {
	TotalUsers        int               `json:"total_users"`
	NewUsers30d       int               `json:"new_users_30d"`
	TotalTransfers    int               `json:"total_transfers"`
	TotalFiles        int               `json:"total_files"`
	TotalStorageB     int64             `json:"total_storage_bytes"`
	TransfersPerDay   []DayStat         `json:"transfers_per_day"`
	UsersPerDay       []DayStat         `json:"users_per_day"`
	StoragePerDay     []StorageDayStat  `json:"storage_per_day"`
	TransferBreakdown TransferBreakdown `json:"transfer_breakdown"`
}

type ServiceHealthItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

type SystemMetrics struct {
	Storage StorageMetrics `json:"storage"`
	Memory  MemoryMetrics  `json:"memory"`
	CPU     CPUMetrics     `json:"cpu"`
}

type StorageMetrics struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type MemoryMetrics struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type CPUMetrics struct {
	UsedPercent float64 `json:"used_percent"`
	Cores       int     `json:"cores"`
}

// ── Config models ────────────────────────────────────────────────────────────

type AuditConfig struct {
	RetentionEnabled bool   `json:"retention_enabled"`
	RetentionDays    int    `json:"retention_days"`
	UpdatedAt        string `json:"updated_at"`
	UpdatedBy        string `json:"updated_by"`
}

type AuditStats struct {
	TotalLogs      int64            `json:"total_logs"`
	Last24h        int64            `json:"last_24h"`
	Last7d         int64            `json:"last_7d"`
	ByAction       map[string]int64 `json:"by_action"`
	OldestLogDate  *string          `json:"oldest_log_date"`
	EligiblePurge  int64            `json:"eligible_purge"`
	TotalSizeBytes *int64           `json:"total_size_bytes"`
}

type AuthLockoutConfig struct {
	MaxFailedAttempts        int    `json:"max_failed_attempts"`
	LockoutDurationMinutes   int    `json:"lockout_duration_minutes"`
	RequireMFA               bool   `json:"require_mfa"`
	RequireEmailVerification bool   `json:"require_email_verification"`
	RegistrationEnabled      bool   `json:"registration_enabled"`
	UpdatedAt                string `json:"updated_at"`
}

type BrandingConfig struct {
	PrimaryColor             string  `json:"primary_color"`
	SecondaryColor           string  `json:"secondary_color"`
	PageBgColor              string  `json:"page_bg_color"`
	SurfaceColor             string  `json:"surface_color"`
	TextColor                string  `json:"text_color"`
	BorderRadius             int     `json:"border_radius"`
	AppName                  string  `json:"app_name"`
	CustomCSS                *string `json:"custom_css"`
	LogoDataURL              *string `json:"logo_data_url"`
	UpdatedAt                string  `json:"updated_at"`
	DmPrimaryColor           *string `json:"dm_primary_color"`
	DmSecondaryColor         *string `json:"dm_secondary_color"`
	DmPageBgColor            *string `json:"dm_page_bg_color"`
	DmSurfaceColor           *string `json:"dm_surface_color"`
	DmTextColor              *string `json:"dm_text_color"`
	EmailSenderName          string  `json:"email_sender_name"`
	EmailSupportEmail        string  `json:"email_support_email"`
	EmailFooterText          string  `json:"email_footer_text"`
	EmailSubjectPrefix       string  `json:"email_subject_prefix"`
	EmailHeaderLink          string  `json:"email_header_link"`
	EmailReplyTo             string  `json:"email_reply_to"`
	EmailButtonColor         string  `json:"email_button_color"`
	EmailButtonTextColor     string  `json:"email_button_text_color"`
	EmailBodyBgColor         string  `json:"email_body_bg_color"`
	EmailCardBgColor         string  `json:"email_card_bg_color"`
	EmailCardBorderColor     string  `json:"email_card_border_color"`
	EmailHeadingColor        string  `json:"email_heading_color"`
	EmailTextColor           string  `json:"email_text_color"`
	SubjectTransferReceived  string  `json:"subject_transfer_received"`
	SubjectPasswordReset     string  `json:"subject_password_reset"`
	SubjectEmailVerification string  `json:"subject_email_verification"`
	SubjectDownloadNotification string `json:"subject_download_notification"`
	SubjectExpiryReminder       string `json:"subject_expiry_reminder"`
	SubjectTransferRevoked      string `json:"subject_transfer_revoked"`
	SubjectRequestSubmission    string `json:"subject_request_submission"`
	CTATransferReceived         string `json:"cta_transfer_received"`
	CTADownloadNotification     string `json:"cta_download_notification"`
	CTAPasswordReset            string `json:"cta_password_reset"`
	CTAEmailVerification        string `json:"cta_email_verification"`
	CTAExpiryReminder           string `json:"cta_expiry_reminder"`
	CTARequestSubmission        string `json:"cta_request_submission"`
	CustomTransferReceived      string `json:"custom_transfer_received"`
	CustomPasswordReset         string `json:"custom_password_reset"`
	CustomEmailVerification     string `json:"custom_email_verification"`
	CustomDownloadNotification  string `json:"custom_download_notification"`
	CustomExpiryReminder        string `json:"custom_expiry_reminder"`
	CustomTransferRevoked       string `json:"custom_transfer_revoked"`
	CustomRequestSubmission     string `json:"custom_request_submission"`
}

type PlatformConfig struct {
	DateFormat           string `json:"date_format"`
	TimeFormat           string `json:"time_format"`
	Timezone             string `json:"timezone"`
	PortalURL            string `json:"portal_url"`
	DownloadURL          string `json:"download_url"`
	LinkProtectionPolicy string `json:"link_protection_policy"`
	UpdatedAt            string `json:"updated_at"`
}

type SmtpSettingsResponse struct {
	Host        string `json:"host"`
	Port        string `json:"port"`
	Username    string `json:"username"`
	PasswordSet bool   `json:"password_set"`
	From        string `json:"from"`
	UseTLS      bool   `json:"use_tls"`
	UpdatedAt   string `json:"updated_at"`
}
