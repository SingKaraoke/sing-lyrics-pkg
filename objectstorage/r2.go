// Package objectstorage wraps the S3-compatible client used to store TTML
// lyrics files in Cloudflare R2, content-addressed by sha256 (see
// docs/ARCHITECTURE.md §5). Same content-addressing scheme as migration/upload_to_r2.py.
package objectstorage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type R2Client struct {
	client *minio.Client
	bucket string
}

func NewR2Client(endpoint, accessKeyID, secretAccessKey, bucket string) (*R2Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil, err
	}
	return &R2Client{client: client, bucket: bucket}, nil
}

// KeyForHash returns the content-addressed object key for a given sha256 hex digest.
func KeyForHash(hexHash string) string {
	return fmt.Sprintf("lyrics/%s.ttml", hexHash)
}

// Exists reports whether an object already exists (used to skip re-uploading
// identical content across different ISRCs).
func (c *R2Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.client.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if errResp := minio.ToErrorResponse(err); errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *R2Client) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := c.client.PutObject(ctx, c.bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Get downloads and returns the full contents of an object. Used server-side
// to parse TTML lyrics files (e.g. to extract plain/synced text for API
// responses) without exposing a presigned URL round trip to the caller.
func (c *R2Client) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

// PresignedGetURL returns a short-lived, signed download URL - the API hands
// this back to clients so bulk file transfer never proxies through the API process.
func (c *R2Client) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := c.client.PresignedGetObject(ctx, c.bucket, key, expiry, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// CheckAccess performs a tiny put+delete round trip so misconfiguration
// (wrong bucket, read-only token, wrong jurisdiction endpoint) fails fast at
// startup instead of on every request (see docs/ARCHITECTURE.md §10).
func (c *R2Client) CheckAccess(ctx context.Context) error {
	const probeKey = "_healthcheck.txt"
	if err := c.Put(ctx, probeKey, strings.NewReader("ok"), 2, "text/plain"); err != nil {
		return fmt.Errorf("R2 access check failed: %w", err)
	}
	return c.client.RemoveObject(ctx, c.bucket, probeKey, minio.RemoveObjectOptions{})
}

// Registry resolves a named storage account (lyrics.storage_account) to the
// R2Client holding its credentials/bucket, so a file can be served from
// whichever of several accounts it was actually uploaded to (see
// migration/upload_to_r2.py's --account flag and docs/ARCHITECTURE.md).
type Registry struct {
	clients        map[string]*R2Client
	defaultAccount string
}

// NewRegistry builds a Registry from one R2Client per configured account.
// defaultAccount is used to resolve legacy/empty storage_account values.
func NewRegistry(clients map[string]*R2Client, defaultAccount string) *Registry {
	return &Registry{clients: clients, defaultAccount: defaultAccount}
}

// Get returns the client for the given account name, falling back to the
// registry's default account when account is empty.
func (r *Registry) Get(account string) (*R2Client, error) {
	if account == "" {
		account = r.defaultAccount
	}
	c, ok := r.clients[account]
	if !ok {
		return nil, fmt.Errorf("no R2 client configured for storage account %q", account)
	}
	return c, nil
}
