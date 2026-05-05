package app

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ibeloyar/gophprofile/internal/config"
	"github.com/ibeloyar/gophprofile/internal/repository/broker"
	"github.com/ibeloyar/gophprofile/internal/repository/s3"
	"github.com/ibeloyar/gophprofile/internal/repository/storage"
	"github.com/ibeloyar/gophprofile/internal/service"
	"github.com/ibeloyar/gophprofile/pkg/logger"
	"go.uber.org/zap"
)

type Server struct {
	*http.Server
}

func NewServer(lg *zap.SugaredLogger, cfg *config.Config) (*Server, error) {
	storageRepo, err := storage.New(cfg.PGConnString)
	if err != nil {
		return nil, err
	}

	publisher, err := broker.NewPublisher(cfg.RabbitURL)
	if err != nil {
		return nil, err
	}

	s3Repo := s3.New(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey)

	r := chi.NewRouter()
	srv := service.New(lg, storageRepo, s3Repo, publisher)

	r.Use(logger.LoggingMiddleware(lg))

	r.Get("/health", srv.Health)
	r.Post("/api/v1/avatars", srv.UploadAvatar)

	return &Server{
		Server: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: r,
		},
	}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {

	return s.Server.Shutdown(ctx)
}
