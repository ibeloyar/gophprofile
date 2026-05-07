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

func New(endpoint, accessKey, secretKey string) (*Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	found, err := client.BucketExists(ctx, avatarsS3BuckedName)
	if err != nil {
		return nil, err
	}
	if !found {
		if err := client.MakeBucket(ctx, avatarsS3BuckedName, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	return &Client{
		client: client,
		bucket: avatarsS3BuckedName,
	}, nil
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

func (c *Client) Upload(ctx context.Context, objectKey, contentType string, data []byte) error {
	_, err := c.client.PutObject(ctx, c.bucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})

	if err != nil {
		return fmt.Errorf("failed to upload object: %v", err)
	}

	return nil
}

func (c *Client) Download(ctx context.Context, objectKey string) ([]byte, string, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object %s: %w", objectKey, err)
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat object %s: %w", objectKey, err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read object %s: %w", objectKey, err)
	}

	return data, info.ContentType, nil
}

func (c *Client) DeleteObjects(ctx context.Context, objectKeys []string) error {
	if len(objectKeys) == 0 {
		return nil
	}

	objectsCh := make(chan minio.ObjectInfo, len(objectKeys))

	for _, key := range objectKeys {
		objectsCh <- minio.ObjectInfo{Key: key}
	}
	close(objectsCh)

	for result := range c.client.RemoveObjects(ctx, c.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			return fmt.Errorf("delete %s: %w", result.ObjectName, result.Err)
		}
	}

	return nil
}
