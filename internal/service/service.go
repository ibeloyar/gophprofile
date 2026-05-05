package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ibeloyar/gophprofile/internal/model"
	"go.uber.org/zap"
)

const (
	maxFileSize      = 10 * 1024 * 1024 // 10MB
	supportedFormats = "image/jpeg,image/png,image/webp"
)

type Storage interface {
	Health() error
	CreateAvatar(ctx context.Context, userID, fileName, mimeType string, sizeBytes int64) (*model.AvatarCreateInfo, error)
	UpdateAvatarS3Key(ctx context.Context, id string, s3Key string) error
}

type S3Storage interface {
	Health() error
	Upload(ctx context.Context, objectKey string, reader io.Reader) error
	Download(ctx context.Context, objectKey string) ([]byte, error)
}

type Publisher interface {
	Health() error

	PublishUpload(ctx context.Context, event *model.AvatarUploadEvent) error
}

type Service struct {
	lg        *zap.SugaredLogger
	storage   Storage
	s3        S3Storage
	publisher Publisher
}

func New(lg *zap.SugaredLogger, storage Storage, s3 S3Storage, publisher Publisher) *Service {
	return &Service{
		lg:        lg,
		storage:   storage,
		s3:        s3,
		publisher: publisher,
	}
}

func (s *Service) Health(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, s.lg, response, http.StatusOK)
}

func (s *Service) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "X-User-ID required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		writeJSON(w, s.lg, model.UploadAvatarSizeError{
			Error:   "File too large",
			MaxSize: maxFileSize,
		}, http.StatusRequestEntityTooLarge)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if !strings.Contains(supportedFormats, contentType) {
		writeJSON(w, s.lg, model.UploadAvatarFormatError{
			Error:   "Invalid file format",
			Details: fmt.Sprintf("invalid file format, supported: %s", supportedFormats),
		}, http.StatusRequestEntityTooLarge)
		return
	}

	if header.Size > maxFileSize {
		writeJSON(w, s.lg, model.UploadAvatarSizeError{
			Error:   "File too large",
			MaxSize: maxFileSize,
		}, http.StatusRequestEntityTooLarge)
		return
	}

	avatar, err := s.storage.CreateAvatar(r.Context(), userID, header.Filename, contentType, header.Size)
	if err != nil {
		s.lg.Error("failed db create avatar", zap.Error(err))
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	objectKey := fmt.Sprintf("%s/%s", userID, avatar.ID)

	err = s.s3.Upload(r.Context(), objectKey, file)
	if err != nil {
		s.lg.Error("failed to s3 upload avatar", zap.Error(err))
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	if err := s.storage.UpdateAvatarS3Key(r.Context(), avatar.ID.String(), objectKey); err != nil {
		s.lg.Error("failed to s3 upload avatar", zap.Error(err))
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	if err := s.publisher.PublishUpload(r.Context(), &model.AvatarUploadEvent{
		AvatarID: avatar.ID.String(),
		UserID:   userID,
		S3Key:    objectKey,
	}); err != nil {
		s.lg.Error("failed to s3 upload avatar", zap.Error(err))
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, s.lg, model.UploadAvatarResponse{
		ID:        avatar.ID.String(),
		UserID:    avatar.UserID,
		URL:       fmt.Sprintf("/api/v1/avatars/%s", avatar.ID),
		Status:    "processing",
		CreatedAt: avatar.CreatedAt,
	}, http.StatusCreated)
}
