package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dmitrygosu/stream-engine/internal/bootstrap"
	"github.com/Dmitrygosu/stream-engine/internal/config"
	"github.com/Dmitrygosu/stream-engine/internal/infrastructure/kafka"
	"github.com/Dmitrygosu/stream-engine/internal/app/authservice"
	"github.com/Dmitrygosu/stream-engine/internal/app/mediaservice"
	"github.com/Dmitrygosu/stream-engine/internal/repository/userrepository"
	"github.com/Dmitrygosu/stream-engine/internal/repository/videorepository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// app container
type App struct {
	cfg        *config.Config
	logger     *zap.Logger
	pgPool     *pgxpool.Pool
	kafkaProd  *kafka.Producer
	httpServer *http.Server
}

func New(cfg *config.Config, logger *zap.Logger) *App {
	return &App{
		cfg:    cfg,
		logger: logger,
	}
}

// init common things
func (a *App) Bootstrap(ctx context.Context) error {
	pool, err := bootstrap.InitDB(ctx, a.cfg.Postgres.URL)
	if err != nil {
		return fmt.Errorf("bootstrap db: %w", err)
	}
	a.pgPool = pool

	a.kafkaProd = kafka.NewProducer(a.cfg.Kafka.Brokers, a.logger)
	
	return nil
}

// clean up
func (a *App) Close() {
	if a.pgPool != nil {
		a.pgPool.Close()
	}
	if a.kafkaProd != nil {
		a.kafkaProd.Close()
	}
}

// start API
func (a *App) RunAPI(ctx context.Context) error {
	// 1. Bootstrap
	if err := a.Bootstrap(ctx); err != nil {
		return err
	}
	defer a.Close()

	// 2. services
	userRepo := userrepository.NewUserRepository(a.pgPool)
	authSvc := authservice.NewAuthService(userRepo, "SUPER_SECRET_KEY_CHANGE_ME")
	authHandler := authservice.NewHandler(authSvc)

	videoRepo := videorepository.NewVideoRepository(a.pgPool)
	
	storageSvc := &mediaservice.LocalStorageService{BaseURL: "https://storage.stream-engine.local"}
	mediaSvc := mediaservice.NewMediaService(videoRepo, a.kafkaProd, storageSvc)
	mediaHandler := mediaservice.NewHandler(mediaSvc)

	// 3. router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	authHandler.RegisterRoutes(router)
	mediaHandler.RegisterRoutes(router)

	router.GET("/health", func(c *gin.Context) {
		if err := a.pgPool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 4. http server
	a.httpServer = &http.Server{
		Addr:    ":" + a.cfg.HTTP.Port,
		Handler: router,
	}

	go func() {
		a.logger.Info("Starting HTTP API", zap.String("port", a.cfg.HTTP.Port))
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 5. graceful exit
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		a.logger.Info("Context cancelled")
	case sig := <-quit:
		a.logger.Info("Received signal", zap.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return a.httpServer.Shutdown(shutdownCtx)
}

// expose deps
func (a *App) GetDeps() (*pgxpool.Pool, *kafka.Producer) {
	return a.pgPool, a.kafkaProd
}

// start worker
func (a *App) RunWorker(ctx context.Context) error {
	// 1. Bootstrap
	if err := a.Bootstrap(ctx); err != nil {
		return err
	}
	defer a.Close()

	// 2. wiring
	videoRepo := videorepository.NewVideoRepository(a.pgPool)
	transcoder := mediaservice.NewFFMPEGTranscoder(a.logger)
	mediaConsumer := mediaservice.NewConsumerHandler(videoRepo, a.kafkaProd, transcoder, a.logger)

	consumer := kafka.NewConsumer(a.cfg.Kafka.Brokers, "video.uploaded", "transcoding-group", a.logger)
	defer consumer.Close()

	// 3. proper shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		a.logger.Info("Shutting down worker...")
		// stop loop:
	}()

	// 4. loop
	a.logger.Info("Worker ready to process events")
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-quit:
			return nil
		default:
			// continue
		}

		msg, err := consumer.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			a.logger.Error("Kafka fetch failed", zap.Error(err))
			time.Sleep(1 * time.Second)
			continue
		}

		if err := mediaConsumer.HandleVideoUploaded(ctx, msg.Key, msg.Value); err != nil {
			a.logger.Error("Failed to handle message", zap.Error(err))
		}

		if err := consumer.Commit(ctx, msg); err != nil {
			a.logger.Error("Kafka commit failed", zap.Error(err))
		}
	}
}
