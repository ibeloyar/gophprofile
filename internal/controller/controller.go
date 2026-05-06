package controller

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ibeloyar/gophprofile/internal/model"
	"go.uber.org/zap"
)

const (
	maxFileSize      = 10 * 1024 * 1024 // 10MB
	supportedFormats = "image/jpeg,image/png,image/webp"
)

type Service interface {
	Health() *model.HealthResponse

	UploadAvatar(ctx context.Context, userID string, avatarFile *model.AvatarFile) (*model.AvatarCreateInfo, error)
	DownloadAvatar(ctx context.Context, avatarID, userID string) ([]byte, string, error)
	GetAvatarMeta(ctx context.Context, avatarID, userID string) (*model.AvatarMeta, error)
	DeleteAvatar(ctx context.Context, avatarID, userID string) error
}

type Controller struct {
	lg      *zap.SugaredLogger
	service Service

	addr string
}

func New(lg *zap.SugaredLogger, service Service) *Controller {
	return &Controller{
		lg:      lg,
		service: service,
	}
}

func (c *Controller) Health(w http.ResponseWriter, r *http.Request) {
	response := c.service.Health()

	writeJSON(w, c.lg, response, http.StatusOK)
}

func (c *Controller) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "X-User-ID required", http.StatusBadRequest)
		return
	}

	avatarFile, err := readAvatarFile(r)
	if err != nil {
		if errors.Is(err, ErrFileRequired) {
			http.Error(w, "file required", http.StatusBadRequest)
			return
		} else if errors.Is(err, ErrFileTooLarge) {
			writeJSON(w, c.lg, model.UploadAvatarSizeError{
				Error:   "File too large",
				MaxSize: maxFileSize,
			}, http.StatusRequestEntityTooLarge)
			return
		} else if errors.Is(err, ErrFileInvalidFormat) {
			writeJSON(w, c.lg, model.UploadAvatarFormatError{
				Error:   "Invalid file format",
				Details: fmt.Sprintf("invalid file format, supported: %s", supportedFormats),
			}, http.StatusBadRequest)
			return
		} else {
			c.lg.Error("read avatar error: ", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	avatar, err := c.service.UploadAvatar(r.Context(), userID, avatarFile)
	if err != nil {
		c.lg.Error("failed db create avatar", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	writeJSON(w, c.lg, model.UploadAvatarResponse{
		ID:        avatar.ID.String(),
		UserID:    avatar.UserID,
		URL:       fmt.Sprintf("/api/v1/avatars/%s", avatar.ID),
		Status:    "processing",
		CreatedAt: avatar.CreatedAt,
	}, http.StatusCreated)
}

func (c *Controller) DownloadAvatar(w http.ResponseWriter, r *http.Request) {
	avatarID := chi.URLParam(r, "avatar_id")

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "X-User-ID required", http.StatusBadRequest)
		return
	}

	fileData, _, err := c.service.DownloadAvatar(r.Context(), avatarID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "key does not exist") {
			writeJSON(w, c.lg, &model.DownloadAvatarNotFoundError{
				Error: "Avatar not found",
			}, http.StatusNotFound)
			return
		}
		c.lg.Error("failed to s3 download avatar", zap.Error(err))
		http.Error(w, "download failed", http.StatusInternalServerError)
		return
	}

	// Установка заголовков
	contentType := http.DetectContentType(fileData)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "max-age=86400")

	// Генерация ETag на основе содержимого (например, через MD5)
	etag := fmt.Sprintf("%x", md5.Sum(fileData))
	w.Header().Set("ETag", etag)

	// Проверка If-None-Match для поддержки кэширования
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(fileData)
}

func (c *Controller) GetAvatarMeta(w http.ResponseWriter, r *http.Request) {
	avatarID := chi.URLParam(r, "avatar_id")

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "X-User-ID required", http.StatusBadRequest)
		return
	}

	avatar, err := c.service.GetAvatarMeta(r.Context(), avatarID, userID)
	if err != nil {
		c.lg.Error("failed to get avatar", zap.Error(err))
		http.Error(w, "failed to get avatar", http.StatusInternalServerError)
		return
	}

	writeJSON(w, c.lg, avatar, http.StatusOK)
}

func (c *Controller) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	avatarID := chi.URLParam(r, "avatar_id")

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "X-User-ID required", http.StatusBadRequest)
		return
	}

	if err := c.service.DeleteAvatar(r.Context(), avatarID, userID); err != nil {
		c.lg.Error("failed to delete avatar", zap.Error(err))
		http.Error(w, "failed to delete avatar", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
