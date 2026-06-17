package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// BlobStore stores and retrieves opaque byte blobs by key (full request logs).
type BlobStore interface {
	PutBlob(ctx context.Context, key string, data []byte) error
	GetBlob(ctx context.Context, key string) ([]byte, error)
}

const minioBucket = "agentdast-scans"

// MinIOStore persists gzipped blobs in a MinIO/S3 bucket.
type MinIOStore struct {
	client *minio.Client
	bucket string
}

// NewMinIOStore constructs a MinIO client from environment variables:
// MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY, MINIO_USE_SSL, MINIO_BUCKET.
func NewMinIOStore() (*MinIOStore, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		bucket = minioBucket
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
		Secure: os.Getenv("MINIO_USE_SSL") == "true",
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	s := &MinIOStore{client: client, bucket: bucket}
	if err := s.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *MinIOStore) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("minio make bucket: %w", err)
		}
	}
	return nil
}

func (s *MinIOStore) PutBlob(ctx context.Context, key string, data []byte) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, &buf, int64(buf.Len()),
		minio.PutObjectOptions{ContentType: "application/gzip"})
	return err
}

func (s *MinIOStore) GetBlob(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	gz, err := gzip.NewReader(obj)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
}
