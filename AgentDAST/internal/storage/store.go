// Package storage persists scan results. The default backend is in-memory;
// MySQL (metadata) and MinIO (full request logs) are used when configured.
package storage

import (
	"context"
	"os"

	"agentdast/pkg/types"
)

// Store persists and retrieves scan results.
type Store interface {
	SaveResult(ctx context.Context, result *types.ScanResult) error
	GetResult(ctx context.Context, scanID string) (*types.ScanResult, error)
	ListResults(ctx context.Context, limit, offset int) ([]*types.ScanResult, error)
	DeleteResult(ctx context.Context, scanID string) error
	Close() error
}

// FromEnv returns a Store configured from environment variables. If MySQL
// settings are present it returns a MySQLStore (optionally backed by MinIO for
// full logs); otherwise it returns an InMemoryStore.
func FromEnv() (Store, error) {
	if os.Getenv("MYSQL_HOST") == "" {
		return NewInMemoryStore(), nil
	}
	var blobs BlobStore
	if os.Getenv("MINIO_ENDPOINT") != "" {
		mb, err := NewMinIOStore()
		if err != nil {
			return nil, err
		}
		blobs = mb
	}
	return NewMySQLStore(blobs)
}
