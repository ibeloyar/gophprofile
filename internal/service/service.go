package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/ibeloyar/gophprofile/internal/model"
	"go.uber.org/zap"
)

type Storage interface {
	Health() error
	Shutdown() error

	CreateAvatar(ctx context.Context, userID, fileName, mimeType string, width, height int, sizeBytes int64) (*model.AvatarCreateInfo, error)
	UpdateAvatarS3Key(ctx context.Context, id string, s3Key string) error
	GetAvatarMeta(ctx context.Context, avatarID, userID string) (*model.AvatarMeta, error)
	GetAvatarByID(ctx context.Context, avatarID, userID string) (*model.Avatar, error)
	SoftDeleteAvatar(ctx context.Context, avatarID, userID string) error
}

type S3Storage interface {
	Health() error

	Upload(ctx context.Context, objectKey, contentType string, data []byte) error
	Download(ctx context.Context, objectKey string) ([]byte, string, error)
}

type Publisher interface {
	Health() error
	Shutdown() error

	PublishUpload(ctx context.Context, event *model.AvatarUploadEvent) error
	PublishDelete(ctx context.Context, event *model.AvatarDeleteEvent) error
}

type Service struct {
	lg        *zap.SugaredLogger
	storage   Storage
	s3        S3Storage
	publisher Publisher
}

// New creates service instance wiring all dependencies.
func New(lg *zap.SugaredLogger, storage Storage, s3 S3Storage, publisher Publisher) *Service {
	return &Service{
		lg:        lg,
		storage:   storage,
		s3:        s3,
		publisher: publisher,
	}
}

// Shutdown closes storage and publisher resources gracefully.
func (s *Service) Shutdown() error {
	if err := s.storage.Shutdown(); err != nil {
		return err
	}
	if err := s.publisher.Shutdown(); err != nil {
		return err
	}

	return nil
}

// Health performs composite health check across all dependencies.
// Returns status object with individual component states.
func (s *Service) Health() *model.HealthResponse {
	response := &model.HealthResponse{
		Postgresql: true,
		Minio:      true,
		RabbitMQ:   true,
	}

	if err := s.storage.Health(); err != nil {
		response.Postgresql = false
	}

	if err := s.s3.Health(); err != nil {
		response.Minio = false
	}

	if err := s.publisher.Health(); err != nil {
		response.RabbitMQ = false
	}

	return response
}

// UploadAvatar implements full upload flow:
// 1. Persist avatar metadata to DB (pending status)
// 2. Upload original file to S3 (userID/avatarID key)
// 3. Update DB with S3 key
// 4. Publish async processing event to RabbitMQ
func (s *Service) UploadAvatar(ctx context.Context, userID string, avatarFile *model.AvatarFile) (*model.AvatarCreateInfo, error) {
	avatar, err := s.storage.CreateAvatar(ctx,
		userID, avatarFile.Filename, avatarFile.ContentType,
		avatarFile.Width, avatarFile.Height, avatarFile.Size,
	)
	if err != nil {
		return nil, errors.New("failed db create avatar")
	}

	objectKey := fmt.Sprintf("%s/%s", userID, avatar.ID)

	err = s.s3.Upload(ctx, objectKey, avatarFile.ContentType, avatarFile.Data)
	if err != nil {
		return nil, errors.New("failed to s3 upload avatar")
	}

	if err := s.storage.UpdateAvatarS3Key(ctx, avatar.ID.String(), objectKey); err != nil {
		return nil, errors.New("failed to storage update S3 key")
	}

	if err := s.publisher.PublishUpload(ctx, &model.AvatarUploadEvent{
		AvatarID: avatar.ID.String(),
		UserID:   userID,
		S3Key:    objectKey,
	}); err != nil {
		return nil, errors.New("failed to s3 upload avatar")
	}

	return avatar, nil
}

// DownloadAvatar downloads original avatar file directly from S3.
// Constructs object key from userID/avatarID.
func (s *Service) DownloadAvatar(ctx context.Context, avatarID, userID string) ([]byte, string, error) {
	objectKey := fmt.Sprintf("%s/%s", userID, avatarID)

	fileData, contentType, err := s.s3.Download(ctx, objectKey)
	if err != nil {
		return nil, "", err
	}

	return fileData, contentType, err
}

// GetAvatarMeta retrieves avatar metadata including processed thumbnails.
func (s *Service) GetAvatarMeta(ctx context.Context, avatarID, userID string) (*model.AvatarMeta, error) {
	avatar, err := s.storage.GetAvatarMeta(ctx, avatarID, userID)
	if err != nil {
		return nil, err
	}

	return avatar, nil
}

// DeleteAvatar implements soft-delete with async cleanup:
// 1. Fetch avatar and authorize owner
// 2. Soft-delete in DB (clear thumbnails, set deleted_at)
// 3. Parse thumbnail keys from JSONB
// 4. Publish delete event for S3 cleanup (original + thumbnails)
func (s *Service) DeleteAvatar(ctx context.Context, avatarID, userID string) error {
	avatar, err := s.storage.GetAvatarByID(ctx, avatarID, userID)
	if err != nil {
		return err
	}
	if avatar == nil {
		return errors.New("not found")
	}

	if avatar.UserID != userID {
		return errors.New("not allowed")
	}

	if err := s.storage.SoftDeleteAvatar(ctx, avatarID, userID); err != nil {
		return err
	}

	thumbnailS3Keys, err := parseThumbnailUrls(userID, avatar.ThumbnailS3Keys)
	if err != nil {
		return err
	}

	s3Keys := make([]string, 0, len(thumbnailS3Keys)+1)
	if avatar.S3Key != "" {
		s3Keys = append(s3Keys, avatar.S3Key)
	}
	s3Keys = append(s3Keys, thumbnailS3Keys...)

	if err := s.publisher.PublishDelete(ctx, &model.AvatarDeleteEvent{
		AvatarID: avatarID,
		S3Keys:   s3Keys,
	}); err != nil {
		return err
	}

	return nil
}

// parseThumbnailUrls converts thumbnail JSONB map to full S3 object keys.
// Prefixes each URL with userID/ (e.g. "user1/avatar_100x100").
func parseThumbnailUrls(userID string, raw *json.RawMessage) ([]string, error) {
	if raw == nil {
		return nil, nil
	}

	var thumbs map[string]string
	if err := json.Unmarshal(*raw, &thumbs); err != nil {
		return nil, fmt.Errorf("unmarshal thumbnails: %w", err)
	}

	urls := make([]string, 0, len(thumbs))
	for _, url := range thumbs {
		if url != "" {
			urls = append(urls, fmt.Sprintf("%s/%s", userID, url))
		}
	}

	return urls, nil
}
