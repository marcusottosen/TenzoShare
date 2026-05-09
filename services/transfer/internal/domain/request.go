package domain

import "time"

// FileRequest is a public upload dropbox created by an authenticated user.
// It generates a slug that can be shared with guests so they can upload files
// without needing an account.
type FileRequest struct {
	ID              string
	OwnerID         string
	OwnerEmail      string // joined from auth.users at read time; not stored in transfer schema
	Slug            string
	Name            string
	Description     string
	AllowedTypes    string // comma-separated MIME prefixes; empty = all types accepted
	MaxSizeMB       int    // 0 = unlimited
	MaxFiles        int    // 0 = unlimited
	ExpiresAt       time.Time
	IsActive        bool
	CreatedAt       time.Time
	SubmissionCount int // populated by ListByOwner only
}

// IsExpired reports whether the request has passed its expiry time or been deactivated.
func (r *FileRequest) IsExpired() bool {
	return !r.IsActive || time.Now().After(r.ExpiresAt)
}

// RequestSubmission records a file uploaded by a guest to a FileRequest.
type RequestSubmission struct {
	ID            string
	RequestID     string
	FileID        string
	Filename      string
	SizeBytes     int64
	SubmitterName string
	Message       string
	SubmitterIP   string
	SubmittedAt   time.Time
}
