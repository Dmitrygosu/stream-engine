package mediaservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Dmitrygosu/stream-engine/internal/model/video"
)

type Transcoder interface {
	Transcode(ctx context.Context, fileID string) (string, time.Duration, error)
}

// SimulatedTranscoder stands in for a real FFmpeg/HLS pipeline: it waits a
// moment and returns a CDN URL for the playlist. It keeps the event flow
// testable end to end without media files.
type SimulatedTranscoder struct {
	delay time.Duration
}

func NewSimulatedTranscoder(delay time.Duration) *SimulatedTranscoder {
	return &SimulatedTranscoder{delay: delay}
}

func (t *SimulatedTranscoder) Transcode(ctx context.Context, fileID string) (string, time.Duration, error) {
	select {
	case <-ctx.Done():
		return "", 0, ctx.Err()
	case <-time.After(t.delay):
	}
	return fmt.Sprintf("https://cdn.stream-engine.local/%s/master.m3u8", fileID), 120 * time.Second, nil
}

type ConsumerHandler struct {
	repo       VideoRepository
	producer   EventProducer
	transcoder Transcoder
	logger     *zap.Logger
}

func NewConsumerHandler(repo VideoRepository, producer EventProducer, transcoder Transcoder, logger *zap.Logger) *ConsumerHandler {
	return &ConsumerHandler{
		repo:       repo,
		producer:   producer,
		transcoder: transcoder,
		logger:     logger,
	}
}

// HandleVideoUploaded processes one video.uploaded event.
//
// It returns an error only for failures worth retrying (database, broker).
// Events that can never succeed — a malformed id, an unknown video — are
// logged and dropped. Kafka delivers at least once, so the handler is
// idempotent: a redelivered event for a published video only re-emits
// video.ready, and a failed video is not processed again.
func (h *ConsumerHandler) HandleVideoUploaded(ctx context.Context, value []byte) error {
	videoID, err := uuid.Parse(string(value))
	if err != nil {
		h.logger.Error("dropping event with malformed video id", zap.ByteString("value", value))
		return nil
	}

	v, err := h.repo.GetByID(ctx, videoID)
	if errors.Is(err, video.ErrVideoNotFound) {
		h.logger.Error("dropping event for unknown video", zap.Stringer("video_id", videoID))
		return nil
	}
	if err != nil {
		return fmt.Errorf("get video: %w", err)
	}

	switch v.Status {
	case video.StatusPublished:
		// Redelivery after a crash between saving and emitting video.ready.
		return h.publishReady(ctx, v)
	case video.StatusFailed:
		h.logger.Info("skipping video that already failed", zap.Stringer("video_id", v.ID))
		return nil
	}

	h.logger.Info("processing video", zap.Stringer("video_id", v.ID))
	v.SetProcessing()
	if err := h.repo.UpdateStatus(ctx, v); err != nil {
		return fmt.Errorf("set processing: %w", err)
	}

	hlsURL, duration, err := h.transcoder.Transcode(ctx, v.ID.String())
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err() // shutting down: the event is redelivered after restart
		}
		h.logger.Error("transcoding failed", zap.Stringer("video_id", v.ID), zap.Error(err))
		v.SetFailed()
		if err := h.repo.UpdateStatus(ctx, v); err != nil {
			return fmt.Errorf("set failed: %w", err)
		}
		return nil
	}

	v.SetPublished(hlsURL, duration)
	if err := h.repo.UpdateStatus(ctx, v); err != nil {
		return fmt.Errorf("set published: %w", err)
	}
	return h.publishReady(ctx, v)
}

func (h *ConsumerHandler) publishReady(ctx context.Context, v *video.Video) error {
	id := []byte(v.ID.String())
	if err := h.producer.Publish(ctx, TopicVideoReady, id, id); err != nil {
		return fmt.Errorf("publish %s: %w", TopicVideoReady, err)
	}
	return nil
}
