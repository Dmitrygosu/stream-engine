package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/Dmitrygosu/stream-engine/internal/modules/media/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type VideoRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Video, error)
	UpdateStatus(ctx context.Context, v *domain.Video) error
}

type EventProducer interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type ConsumerHandler struct {
	repo     VideoRepository
	producer EventProducer
	logger   *zap.Logger
}

func NewConsumerHandler(repo VideoRepository, producer EventProducer, logger *zap.Logger) *ConsumerHandler {
	return &ConsumerHandler{
		repo:     repo,
		producer: producer,
		logger:   logger,
	}
}

func (h *ConsumerHandler) HandleVideoUploaded(ctx context.Context, key, value []byte) error {
	videoIDStr := string(value)
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		h.logger.Error("failed to parse video id", zap.Error(err), zap.String("value", videoIDStr))
		return nil // bad data, no retry
	}

	h.logger.Info("Processing video", zap.String("video_id", videoIDStr))

	video, err := h.repo.GetByID(ctx, videoID)
	if err != nil {
		return fmt.Errorf("get video: %w", err)
	}

	// 1. status -> processing
	video.SetProcessing()
	if err := h.repo.UpdateStatus(ctx, video); err != nil {
		return fmt.Errorf("set processing: %w", err)
	}

	// 2. fake work (transcoding)
	h.logger.Info("Transcoding started...", zap.String("video_id", videoIDStr))
	time.Sleep(2 * time.Second) // nap time
	h.logger.Info("Transcoding finished", zap.String("video_id", videoIDStr))

	// 3. status -> published
	// fake url
	hlsURL := fmt.Sprintf("https://cdn.stream-engine.local/%s/master.m3u8", video.ID)
	duration := 120 * time.Second

	video.SetPublished(hlsURL, duration)
	if err := h.repo.UpdateStatus(ctx, video); err != nil {
		return fmt.Errorf("set published: %w", err)
	}

	// 4. event: video.ready
	if err := h.producer.Publish(ctx, "video.ready", []byte(video.ID.String()), []byte(video.ID.String())); err != nil {
		h.logger.Error("failed to publish video.ready", zap.Error(err))
		// db saved, ignore kafka fail
	}

	return nil
}
