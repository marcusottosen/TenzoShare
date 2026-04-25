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
