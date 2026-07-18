package storage

import (
	"context"
	"io"
)

// Store abstracts object storage so implementations (MinIO locally,
// Azure Blob in prod) are swappable behind the same interface.
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
