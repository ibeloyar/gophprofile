package broker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ibeloyar/gophprofile/internal/model"
	"github.com/ibeloyar/gophprofile/pkg/resizer"
	"go.uber.org/zap"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	workerName = "gophprofile-worker"
)

type Storage interface {
	UpdateProcessingStatus(ctx context.Context, avatarID string, status model.ProcessingOp) error
	SetThumbnailsData(ctx context.Context, avatarID string, avatarThumbnails []byte) error
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
			c.lg.Errorf("unmarshal upload event: %v", err)
			msg.Nack(false, false)
			continue
		}

		if err := c.UploadHandler(context.Background(), &event); err != nil {
			c.lg.Errorf("upload handler error: %v", err)
			msg.Nack(false, false)
			continue
		}

		msg.Ack(false)
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
			c.lg.Errorf("unmarshal delete event: %v", err)
			msg.Nack(false, false)
			continue
		}

		if err := c.DeleteHandler(context.Background(), &event); err != nil {
			c.lg.Errorf("delete handler error: %v", err)
			msg.Nack(false, false)
			continue
		}

		msg.Ack(false)
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
