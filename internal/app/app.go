package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/ibeloyar/gophprofile/internal/config"
	"github.com/ibeloyar/gophprofile/internal/repository/broker"
	"github.com/ibeloyar/gophprofile/internal/repository/s3"
	"github.com/ibeloyar/gophprofile/internal/repository/storage"
	"github.com/ibeloyar/gophprofile/internal/service"
	"github.com/ibeloyar/gophprofile/pkg/logger"
)

const (
	appName = "gophprofile"

	shutdownTimeout = 10 * time.Second
)

func Run(cfg *config.Config) error {
	lg, err := logger.New()
	if err != nil {
		return err
	}
	defer lg.Sync()

	storageRepo, err := storage.New(cfg.PGConnString)
	if err != nil {
		return err
	}

	publisher, err := broker.NewPublisher(lg, cfg.RabbitURL)
	if err != nil {
		return err
	}

	if err := publisher.Init(); err != nil {
		return err
	}

	s3Repo, err := s3.New(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey)
	if err != nil {
		return err
	}

	srv := service.New(lg, storageRepo, s3Repo, publisher)

	server, err := NewServer(lg, cfg.HTTPAddr, srv)
	if err != nil {
		return err
	}

	go func() {
		lg.Info(fmt.Sprintf("%s starting http server on address: %s", appName, cfg.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lg.Fatal(err)
		}
	}()

	// Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	lg.Info("shutting down app...")

	stopChan := make(chan struct{})

	go func() {
		if err := srv.Shutdown(); err != nil {
			lg.Errorf("shutdown (service) error: %s", err)
		}

		if err := server.Shutdown(shutdownCtx); err != nil {
			lg.Errorf("shutdown (server) error: %s", err)
		}

		stopChan <- struct{}{}
	}()

	select {
	case <-stopChan:
		lg.Info("shutting down gracefully")
	case <-time.NewTicker(shutdownTimeout).C:
		lg.Error("shutting down timed out error")
	}

	return nil
}
