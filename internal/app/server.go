package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ibeloyar/gophprofile/internal/controller"
	"github.com/ibeloyar/gophprofile/internal/service"
	"github.com/ibeloyar/gophprofile/pkg/logger"
	"go.uber.org/zap"
)

func NewServer(lg *zap.SugaredLogger, addr string, service *service.Service) (*http.Server, error) {
	r := chi.NewRouter()
	c := controller.New(lg, service)

	r.Use(logger.LoggingMiddleware(lg))

	r.Get("/health", c.Health)
	r.Post("/api/v1/avatars", c.UploadAvatar)
	r.Get("/api/v1/avatars/{avatar_id}", c.DownloadAvatar)
	r.Get("/api/v1/avatars/{avatar_id}/metadata", c.GetAvatarMeta)
	r.Delete("/api/v1/avatars/{avatar_id}", c.DeleteAvatar)

	return &http.Server{
		Addr:    addr,
		Handler: r,
	}, nil
}
