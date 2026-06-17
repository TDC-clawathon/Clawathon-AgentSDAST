// Package store wraps the MinIO/S3 object store used to exchange artifacts with
// the Manager.
package store

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc     *minio.Client
	bucket string
}

func New(endpoint, access, secret, bucket string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(access, secret, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &Client{mc: mc, bucket: bucket}, nil
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	ok, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
	}
	return nil
}

// DownloadPrefix mirrors every object under prefix/ into destDir, preserving the
// path layout below the prefix. Returns the number of objects fetched.
func (c *Client) DownloadPrefix(ctx context.Context, prefix, destDir string) (int, error) {
	prefix = strings.TrimSuffix(prefix, "/") + "/"
	n := 0
	for obj := range c.mc.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return n, obj.Err
		}
		rel := strings.TrimPrefix(obj.Key, prefix)
		if rel == "" || strings.HasSuffix(obj.Key, "/") {
			continue
		}
		local := filepath.Join(destDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
			return n, err
		}
		if err := c.mc.FGetObject(ctx, c.bucket, obj.Key, local, minio.GetObjectOptions{}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// UploadFile stores localPath at objectKey with the given content type.
func (c *Client) UploadFile(ctx context.Context, localPath, objectKey, contentType string) error {
	_, err := c.mc.FPutObject(ctx, c.bucket, objectKey, localPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// GetText returns the full content of an object as a string (used by /result).
func (c *Client) GetText(ctx context.Context, objectKey string) (string, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	b, err := io.ReadAll(obj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.mc.BucketExists(ctx, c.bucket)
	return err
}
