package domain

import "time"

// Transfer represents a file-sharing bundle created by an owner.
type Transfer struct {
	ID             string
	OwnerID        string
	SenderEmail    string // email of the owner at creation time, shown to recipients
	Name           string // human-readable label, required
	Description    string // optional longer note
	RecipientEmail string // empty = public link
	Slug           string // short URL token
	PasswordHash   string // empty = no password
	MaxDownloads   int    // 0 = unlimited; limit is enforced per individual file
	DownloadCount  int    // informational grand total; not used for limit enforcement
	FileCount      int    // number of files; populated by repo queries
	IsExhausted    bool   // true when every file has reached MaxDownloads; populated by repo
	ExpiresAt      *time.Time // always set; "never" is not permitted
	IsRevoked      bool
	CreatedAt      time.Time
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
