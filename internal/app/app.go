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

	server, err := NewServer(lg, cfg)
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

	lg.Info("shutting down server...")

	stopChan := make(chan struct{})

	go func() {
		if err := server.Shutdown(shutdownCtx); err != nil {
			lg.Errorf("shutdown error: %s", err)
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
