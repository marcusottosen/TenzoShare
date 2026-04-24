package domain

import "time"

type File struct {
	ID          string
	OwnerID     string
	ObjectKey   string
	Filename    string
	ContentType string
	SizeBytes   int64
	IsEncrypted bool
	CreatedAt   time.Time
	DeletedAt   *time.Time
}
