package domain

import "time"

// Transfer represents a file-sharing bundle created by an owner.
type Transfer struct {
	ID               string
	OwnerID          string
	SenderEmail      string     // email of the owner at creation time, shown to recipients
	Name             string     // human-readable label, required
	Description      string     // optional longer note
	RecipientEmail   string     // empty = public link
	Slug             string     // short URL token
	PasswordHash     string     // empty = no password
	MaxDownloads     int        // 0 = unlimited; limit is enforced per individual file (also "max views" for ViewOnly)
	DownloadCount    int        // informational grand total; not used for limit enforcement (also "view count" for ViewOnly)
	ViewOnly         bool       // true = file is served inline only; no download button shown to recipient
	FileCount        int        // number of files; populated by repo queries
	TotalSizeBytes   int64      // sum of size_bytes from storage.files; populated by repo queries
	IsExhausted      bool       // true when every file has reached MaxDownloads; populated by repo
	ExpiresAt        *time.Time // always set; "never" is not permitted
	IsRevoked        bool
	ReminderSentAt   *time.Time // non-nil when an expiry reminder email has been sent
	NotifyOnDownload bool       // true = email the owner when a recipient downloads a file
	CreatedAt        time.Time
}

// Status returns the current lifecycle state: "revoked", "exhausted", "expired", or "active".
// Priority: revoked > exhausted > expired > active.
// IsExhausted is populated by the repository (DB subquery on file_download_counts).
func (t *Transfer) Status() string {
	if t.IsRevoked {
		return "revoked"
	}
	if t.MaxDownloads > 0 && t.IsExhausted {
		return "exhausted"
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return "expired"
	}
	return "active"
}

// TransferFile is a row in transfer.transfer_files.
type TransferFile struct {
	TransferID string
	FileID     string
}

// RecipientToken is a per-recipient magic link token for email-shared transfers.
// The raw token is sent once (embedded in the email link as ?rt=<token>).
// Only the SHA-256 hash is persisted.
type RecipientToken struct {
	ID         string
	TransferID string
	Email      string
	TokenHash  string // hex(SHA-256(raw token))
	ExpiresAt  time.Time
	CreatedAt  time.Time
}
