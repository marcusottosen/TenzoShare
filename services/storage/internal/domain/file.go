package domain

import "time"

type File struct {
	ID           string
	OwnerID      string
	ObjectKey    string
	Filename     string
	ContentType  string
	SizeBytes    int64
	IsEncrypted  bool
	EncryptionIV []byte // 12-byte AES-256-GCM nonce; nil for unencrypted files
	CreatedAt    time.Time
	DeletedAt    *time.Time
}

// UserStorageUsage aggregates storage consumption for a single user.
type UserStorageUsage struct {
	UserID     string
	FileCount  int64
	TotalBytes int64
}

// StorageConfig is the singleton storage-policy row from storage.storage_settings.
type StorageConfig struct {
	QuotaEnabled         bool
	QuotaBytesPerUser    int64
	MaxUploadSizeBytes   int64
	RetentionEnabled     bool
	RetentionDays        int // days after last share expires before file is deleted
	OrphanRetentionDays  int // days for files that were never shared
	// TestMode disables the HTTPS-only requirement for uploads.
	// Should only be enabled in development / test environments.
	TestMode             bool
}

// FileWithShareInfo extends File with live share/retention context for the user portal.
type FileWithShareInfo struct {
	File
	ShareCount         int
	ActiveShares       int
	LastShareExpiresAt *time.Time
	// AutoDeleteAt is the computed date after which the file becomes eligible for
	// automatic deletion. Nil when retention is disabled or the file is protected by
	// at least one active (non-expiring) share.
	AutoDeleteAt *time.Time
}

// FileToDelete is a row returned by the retention cleanup query.
type FileToDelete struct {
	ID        string
	ObjectKey string
	OwnerID   string
	Filename  string
	SizeBytes int64
	Reason    string // 'retention_expired' | 'orphan_expired'
}
