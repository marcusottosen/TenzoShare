// Package storage defines the StorageBackend interface that all storage
// implementations (MinIO, AWS S3, Azure Blob, GCS) must satisfy.
// Concrete implementations live in services/storage/internal/backends/.
package storage

import (
	"context"
	"io"
	"time"
)

// Backend is the S3-compatible storage abstraction.
// All file I/O in TenzoShare goes through this interface.
type Backend interface {
	// Upload stores data at the given key. size is the total byte count
	// (-1 if unknown). contentType is the MIME type (e.g. "application/octet-stream").
	Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error

	// Download returns a ReadCloser for the object at key.
	// The caller must close the reader when done.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object at key. Returns nil if already absent.
	Delete(ctx context.Context, key string) error

	// GetPresignedURL returns a time-limited URL for direct client access.
	GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)

	// Exists returns true if an object with the given key exists.
	Exists(ctx context.Context, key string) (bool, error)
}
