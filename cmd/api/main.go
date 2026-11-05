package main

// Entry point for the HTTP API Server.
// Handles user requests (Auth, Video Management).

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
	if err := cfg.Validate(true); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	l := logger.New(cfg.Logger.Level)
	defer func() { _ = l.Sync() }()

	application := app.New(cfg, l)

	if err := application.RunAPI(context.Background()); err != nil {
		l.Fatal("Application failed", zap.Error(err))
	}
}
