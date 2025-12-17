package main

// Entry point for the Background Worker.
// Consumes Kafka events to process heavy tasks (Simulated Transcoding).

import (
	"context"
	"log"

	"github.com/Dmitrygosu/stream-engine/internal/app"
	"github.com/Dmitrygosu/stream-engine/internal/config"
	"github.com/Dmitrygosu/stream-engine/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}

	l := logger.New(cfg.Logger.Level)
	defer l.Sync()

	l.Info("Worker starting...", zap.String("version", cfg.App.Version))

	application := app.New(cfg, l)
	
	if err := application.RunWorker(context.Background()); err != nil {
		l.Fatal("Worker failed", zap.Error(err))
	}
}
