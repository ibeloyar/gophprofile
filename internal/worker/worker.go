package worker

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/ibeloyar/gophprofile/internal/config"
	"github.com/ibeloyar/gophprofile/internal/repository/broker"
	"github.com/ibeloyar/gophprofile/internal/repository/s3"
	"github.com/ibeloyar/gophprofile/internal/repository/storage"
	"github.com/ibeloyar/gophprofile/pkg/logger"
)

const (
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

	s3Repo, err := s3.New(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey)
	if err != nil {
		return err
	}

	consumer, err := broker.NewConsumer(lg, cfg.RabbitURL, storageRepo, s3Repo)
	if err != nil {
		return err
	}

	go func() {
		if err = consumer.Run(); err != nil {
			lg.Fatal("starting consumer failed: %v", err)
		}
	}()

	// Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	lg.Info("shutting down worker...")

	stopChan := make(chan struct{})

	go func() {
		if err := consumer.Shutdown(); err != nil {
			lg.Errorf("shutdown worker (consumer) error: %s", err)
		}
		if err := storageRepo.Shutdown(); err != nil {
			lg.Errorf("shutdown worker (storage) error: %s", err)
		}

		stopChan <- struct{}{}
	}()

	select {
	case <-stopChan:
		lg.Info("shutting down worker gracefully")
	case <-time.NewTicker(shutdownTimeout).C:
		lg.Error("shutting down worker timed out error")
	}

	return nil
}
