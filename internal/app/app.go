// Package app wires the services together and runs the API and the worker.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Dmitrygosu/stream-engine/internal/app/authservice"
	"github.com/Dmitrygosu/stream-engine/internal/app/mediaservice"
	"github.com/Dmitrygosu/stream-engine/internal/auth"
	"github.com/Dmitrygosu/stream-engine/internal/bootstrap"
	"github.com/Dmitrygosu/stream-engine/internal/config"
	"github.com/Dmitrygosu/stream-engine/internal/httpx"
	"github.com/Dmitrygosu/stream-engine/internal/infrastructure/kafka"
	"github.com/Dmitrygosu/stream-engine/internal/repository/userrepository"
	"github.com/Dmitrygosu/stream-engine/internal/repository/videorepository"
)

const (
	shutdownTimeout  = 10 * time.Second
	readinessTimeout = 2 * time.Second
	transcodingGroup = "transcoding-group"
)

type App struct {
	cfg       *config.Config
	logger    *zap.Logger
	pgPool    *pgxpool.Pool
	kafkaProd *kafka.Producer
}

func New(cfg *config.Config, logger *zap.Logger) *App {
	return &App{cfg: cfg, logger: logger}
}

func (a *App) bootstrap(ctx context.Context) error {
	pool, err := bootstrap.InitDB(ctx, a.cfg.Postgres.URL)
	if err != nil {
		return fmt.Errorf("bootstrap db: %w", err)
	}
	a.pgPool = pool
	a.kafkaProd = kafka.NewProducer(a.cfg.Kafka.Brokers, a.logger)
	return nil
}

func (a *App) close() {
	if a.kafkaProd != nil {
		if err := a.kafkaProd.Close(); err != nil {
			a.logger.Error("close kafka producer", zap.Error(err))
		}
	}
	if a.pgPool != nil {
		a.pgPool.Close()
	}
}

// RunAPI serves HTTP until ctx is canceled or SIGINT/SIGTERM arrives, then
// lets in-flight requests finish.
func (a *App) RunAPI(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := a.bootstrap(ctx); err != nil {
		return err
	}
	defer a.close()

	// Only the API applies migrations; the worker expects the schema to exist.
	if err := bootstrap.Migrate(ctx, a.pgPool); err != nil {
		return err
	}

	server := &http.Server{
		Addr:    ":" + a.cfg.HTTP.Port,
		Handler: a.router(),
		// Without a header timeout a slow client can hold connections forever (Slowloris).
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		a.logger.Info("Starting HTTP API", zap.String("addr", server.Addr))
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		a.logger.Info("Shutting down API")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (a *App) router() *gin.Engine {
	tokens := auth.NewTokens([]byte(a.cfg.Auth.JWTSecret), a.cfg.Auth.Audience, a.cfg.Auth.TokenTTL)

	authHandler := authservice.NewHandler(
		authservice.NewAuthService(userrepository.NewUserRepository(a.pgPool), tokens))

	storage := &mediaservice.LocalStorageService{BaseURL: "https://storage.stream-engine.local"}
	mediaHandler := mediaservice.NewHandler(
		mediaservice.NewMediaService(videorepository.NewVideoRepository(a.pgPool), a.kafkaProd, storage))

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), httpx.RequestLogger(a.logger))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
		defer cancel()
		if err := a.pgPool.Ping(ctx); err != nil {
			_ = c.Error(err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler.RegisterRoutes(router)
	mediaHandler.RegisterRoutes(router, auth.RequireAuth(tokens))
	return router
}

// RunWorker processes video.uploaded events until ctx is canceled or
// SIGINT/SIGTERM arrives. The event being processed at shutdown is not
// committed and is redelivered after a restart.
func (a *App) RunWorker(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := a.bootstrap(ctx); err != nil {
		return err
	}
	defer a.close()

	handler := mediaservice.NewConsumerHandler(
		videorepository.NewVideoRepository(a.pgPool),
		a.kafkaProd,
		mediaservice.NewSimulatedTranscoder(2*time.Second),
		a.logger,
	)
	consumer := kafka.NewConsumer(a.cfg.Kafka.Brokers, mediaservice.TopicVideoUploaded, transcodingGroup, a.logger)
	defer func() {
		if err := consumer.Close(); err != nil {
			a.logger.Error("close kafka consumer", zap.Error(err))
		}
	}()

	a.logger.Info("Worker ready to process events")
	err := mediaservice.NewWorker(consumer, handler, a.logger, mediaservice.DefaultRetryBackoff).Run(ctx)
	a.logger.Info("Worker stopped")
	return err
}
