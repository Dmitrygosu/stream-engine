package main

// Entry point for the Background Worker.
// Consumes Kafka events to process heavy tasks (Simulated Transcoding).

import (
	"context"
	"log"

	"go.uber.org/zap"

	"github.com/Dmitrygosu/stream-engine/internal/app"
	"github.com/Dmitrygosu/stream-engine/internal/config"
	"github.com/Dmitrygosu/stream-engine/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}
	if err := cfg.Validate(false); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	l := logger.New(cfg.Logger.Level)
	defer func() { _ = l.Sync() }()

	l.Info("Worker starting...", zap.String("version", cfg.App.Version))

	application := app.New(cfg, l)

	if err := application.RunWorker(context.Background()); err != nil {
		l.Fatal("Worker failed", zap.Error(err))
	}
}
