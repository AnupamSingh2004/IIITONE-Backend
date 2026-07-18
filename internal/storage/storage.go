package storage

import (
	"context"
	"io"
)

// Store abstracts object storage so implementations (MinIO locally,
// Azure Blob in prod) are swappable behind the same interface.
type Store interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	// Get returns a reader for the object. A nil error here does NOT guarantee
	// the object exists: implementations may return the reader before any
	// network request has been made, surfacing a not-found error only on the
	// first Read() call rather than from Get() itself.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
