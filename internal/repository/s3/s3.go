package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	avatarsS3BuckedName = "avatars"
)

type Client struct {
	client *minio.Client
	bucket string
}

func New(endpoint, accessKey, secretKey string) *Client {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil
	}

	return &Client{
		client: client,
		bucket: avatarsS3BuckedName,
	}
}

func (c *Client) Upload(ctx context.Context, objectKey string, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	_, err = c.client.PutObject(ctx, c.bucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})

	if err != nil {
		return fmt.Errorf("failed to upload object: %v", err)
	}

	return nil
}

func (c *Client) Download(ctx context.Context, objectKey string) ([]byte, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", objectKey, err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to read object %s: %w", objectKey, err)
	}

	return data, nil
}

func (c *Client) Health() error {
	cancel, err := c.client.HealthCheck(5 * time.Second)
	if err != nil {
		return err
	}
	defer cancel()

	if c.client.IsOffline() {
		return fmt.Errorf("minio client is offline")
	}

	return nil
}
