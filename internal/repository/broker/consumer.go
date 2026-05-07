package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ibeloyar/gophprofile/internal/model"
	"github.com/ibeloyar/gophprofile/pkg/resizer"
	"go.uber.org/zap"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	workerName = "gophprofile-worker"

	maxHandleRetries = 3
)

type Storage interface {
	UpdateProcessingStatus(ctx context.Context, avatarID string, status model.ProcessingOp) error
	SetThumbnailsData(ctx context.Context, avatarID string, avatarThumbnails []byte) error
	DeleteAvatarThumbnailsData(ctx context.Context, avatarID string) error
	AvatarResizeIsProcessed(ctx context.Context, avatarID string) (bool, error)
	CheckAvatarThumbnailKeysIsDeleted(ctx context.Context, avatarID string) (bool, error)
}

type S3Storage interface {
	Upload(ctx context.Context, objectKey, contentType string, data []byte) error
	Download(ctx context.Context, objectKey string) ([]byte, string, error)
	DeleteObjects(ctx context.Context, objectKeys []string) error
}

type Consumer struct {
	lg      *zap.SugaredLogger
	conn    *amqp.Connection
	channel *amqp.Channel
	storage Storage
	s3      S3Storage
}

// NewConsumer creates and initializes RabbitMQ consumer connection.
func NewConsumer(lg *zap.SugaredLogger, url string, storage Storage, s3 S3Storage) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Consumer{
		lg:      lg,
		conn:    conn,
		channel: ch,
		storage: storage,
		s3:      s3,
	}, nil
}

// Run declares queues/bindings, sets QoS, starts upload/delete goroutines.
// Blocks until Shutdown called. Logs worker startup.
func (c *Consumer) Run() error {
	if _, err := c.channel.QueueDeclare(uploadQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := c.channel.QueueDeclare(deleteQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err := c.channel.QueueBind(uploadQueue, uploadKey, exchangeName, false, nil); err != nil {
		return err
	}
	if err := c.channel.QueueBind(deleteQueue, deleteKey, exchangeName, false, nil); err != nil {
		return err
	}
	if err := c.channel.Qos(10, 0, false); err != nil {
		return err
	}

	go c.handleUpload()
	go c.handleDelete()

	c.lg.Info(fmt.Sprintf("%s running...", workerName))

	return nil
}

// Shutdown closes RabbitMQ channel and connection gracefully.
func (c *Consumer) Shutdown() error {
	if err := c.channel.Close(); err != nil {
		return err
	}

	return c.conn.Close()
}

// handleUpload consumes from upload queue, processes avatar thumbnails.
func (c *Consumer) handleUpload() {
	msgs, err := c.channel.Consume(uploadQueue, "upload-worker", false, false, false, false, nil)
	if err != nil {
		c.lg.Errorf("upload consume: %v", err)
		return
	}

	for msg := range msgs {
		var event model.AvatarUploadEvent

		if err := json.Unmarshal(msg.Body, &event); err != nil {
			c.lg.Errorf("unmarshal: %v", err)
			msg.Nack(false, false)
			continue
		}

		if alreadyProcessed, err := c.storage.AvatarResizeIsProcessed(context.Background(), event.AvatarID); err != nil {
			c.lg.Errorf("idempotency check failed: %v", err)
			msg.Nack(false, false)
			continue
		} else if alreadyProcessed {
			c.lg.Infow("duplicate message skipped", "avatar_id", event.AvatarID, "message_id", event.MessageID)
			msg.Ack(false)
			continue
		}

		// Retry with exponential backoff
		for attempt := 1; attempt <= maxHandleRetries; attempt++ {
			if err := c.UploadHandler(context.Background(), &event); err != nil {
				backoff := time.Duration(1<<uint(attempt)) * time.Second // 2s, 4s, 8s

				c.lg.Warnw("upload attempt failed", "attempt", attempt, "error", err, "backoff", backoff)

				if attempt == maxHandleRetries {
					// Final failure → Dead Letter Queue (DLQ)
					msg.Nack(false, true) // requeue=false
					break
				}
				time.Sleep(backoff)
				continue
			}

			// Success: mark idempotent + ACK
			if err := c.storage.UpdateProcessingStatus(context.Background(), event.AvatarID, model.ProcessingOpCompleted); err != nil {
				c.lg.Errorw("mark processed failed", "error", err)
				msg.Nack(false, false)
				break
			}

			msg.Ack(false)

			break
		}
	}
}

// handleDelete consumes from delete queue, removes S3 objects.
func (c *Consumer) handleDelete() {
	msgs, err := c.channel.Consume(deleteQueue, "delete-worker", false, false, false, false, nil)
	if err != nil {
		c.lg.Errorf("delete consume: %v", err)
		return
	}

	for msg := range msgs {
		var event model.AvatarDeleteEvent

		if err := json.Unmarshal(msg.Body, &event); err != nil {
			c.lg.Errorf("failed to unmarshal delete event: %v", err)
			msg.Nack(false, false)
			continue
		}

		if alreadyDeleted, err := c.storage.CheckAvatarThumbnailKeysIsDeleted(context.Background(), event.AvatarID); err != nil {
			c.lg.Errorf("idempotency check failed: %v", err)
			msg.Nack(false, false)
			continue
		} else if alreadyDeleted {
			c.lg.Infow("duplicate delete skipped", "avatar_id", event.AvatarID, "message_id", event.MessageID)
			msg.Ack(false)
			continue
		}

		// Retry with exponential backoff
		const maxRetries = 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if err := c.DeleteHandler(context.Background(), &event); err != nil {
				backoff := time.Duration(1<<uint(attempt)) * time.Second // 2s, 4s, 8s

				c.lg.Warnw("delete attempt failed",
					"attempt", attempt, "max_retries", maxRetries,
					"error", err, "backoff", backoff,
					"avatar_id", event.AvatarID)

				if attempt == maxRetries {
					c.lg.Errorw("max retries exceeded, sending to DLQ", "avatar_id", event.AvatarID)
					msg.Nack(false, true) // requeue=false → DLQ
					break
				}
				time.Sleep(backoff)
				continue
			}

			if markErr := c.storage.DeleteAvatarThumbnailsData(context.Background(), event.AvatarID); markErr != nil {
				c.lg.Errorw("failed to mark delete processed", "error", markErr, "avatar_id", event.AvatarID)
				msg.Nack(false, false)
				break
			}

			c.lg.Infow("delete processed successfully",
				"avatar_id", event.AvatarID,
				"s3_keys_deleted", len(event.S3Keys),
				"message_id", event.MessageID)

			msg.Ack(false)

			break
		}
	}
}

// UploadHandler processes avatar upload event:
// 1. Updates status to Processing
// 2. Downloads original from S3
// 3. Generates 100x100 and 300x300 thumbnails
// 4. Uploads thumbnails to S3
// 5. Stores thumbnails metadata in DB
func (c *Consumer) UploadHandler(ctx context.Context, event *model.AvatarUploadEvent) error {
	if err := c.storage.UpdateProcessingStatus(ctx, event.AvatarID, model.ProcessingOpProcessing); err != nil {
		return err
	}

	originalImage, _, err := c.s3.Download(ctx, event.S3Key)
	if err != nil {
		return err
	}

	thumbnails := []struct {
		width  int
		height int
	}{
		{100, 100},
		{300, 300},
	}

	avatarThumbnailsMap := make(map[string]string)

	for _, thumb := range thumbnails {
		thumbnailImageData, err := resizer.Resize(originalImage, thumb.width, thumb.height)
		if err != nil {
			if err := c.storage.UpdateProcessingStatus(ctx, event.AvatarID, model.ProcessingOpFailed); err != nil {
				return err
			}
			return err
		}

		objectKey := fmt.Sprintf("%s/%s_%dx%d", event.UserID, event.AvatarID, thumb.width, thumb.height)
		if err := c.s3.Upload(ctx, objectKey, "image/jpeg", thumbnailImageData); err != nil {
			if err := c.storage.UpdateProcessingStatus(ctx, event.AvatarID, model.ProcessingOpFailed); err != nil {
				return err
			}
			return err
		}

		thumbnailKey := fmt.Sprintf("%dx%d", thumb.width, thumb.height)
		thumbnailUrl := fmt.Sprintf("%s_%dx%d", event.AvatarID, thumb.width, thumb.height)
		avatarThumbnailsMap[thumbnailKey] = thumbnailUrl
	}

	avatarThumbnails, err := json.Marshal(avatarThumbnailsMap)
	if err != nil {
		return fmt.Errorf("marshal thumbnails: %w", err)
	}

	return c.storage.SetThumbnailsData(ctx, event.AvatarID, avatarThumbnails)
}

// DeleteHandler removes avatar files from S3 storage.
func (c *Consumer) DeleteHandler(ctx context.Context, event *model.AvatarDeleteEvent) error {
	if err := c.s3.DeleteObjects(ctx, event.S3Keys); err != nil {
		return err
	}

	return nil
}
