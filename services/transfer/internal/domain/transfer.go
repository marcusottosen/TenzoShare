package domain

import "time"

// Transfer represents a file-sharing bundle created by an owner.
type Transfer struct {
	ID             string
	OwnerID        string
	Name           string // human-readable label, required
	Description    string // optional longer note
	RecipientEmail string // empty = public link
	Slug           string // short URL token
	PasswordHash   string // empty = no password
	MaxDownloads   int    // 0 = unlimited
	DownloadCount  int
	ExpiresAt      *time.Time // always set; "never" is not permitted
	IsRevoked      bool
	CreatedAt      time.Time
}

// Status returns the current lifecycle state: "revoked", "expired", or "active".
func (t *Transfer) Status() string {
	if t.IsRevoked {
		return "revoked"
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
