package mediaservice

import (
	"context"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MessageSource is the part of a Kafka consumer the worker needs.
type MessageSource interface {
	Fetch(ctx context.Context) (kafkago.Message, error)
	Commit(ctx context.Context, msg kafkago.Message) error
}

// DefaultRetryBackoff is the delay before each retry of a failed event.
var DefaultRetryBackoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}

// Worker consumes video.uploaded events with at-least-once semantics.
type Worker struct {
	source  MessageSource
	handler *ConsumerHandler
	logger  *zap.Logger
	backoff []time.Duration
}

func NewWorker(source MessageSource, handler *ConsumerHandler, logger *zap.Logger, backoff []time.Duration) *Worker {
	return &Worker{source: source, handler: handler, logger: logger, backoff: backoff}
}

// Run handles events until ctx is canceled.
//
// An event is committed once it has been handled, or once its retries are
// exhausted (it is then logged; a dead-letter topic would be the next step).
// An event interrupted by shutdown is not committed, so Kafka redelivers it
// after a restart.
func (w *Worker) Run(ctx context.Context) error {
	for {
		msg, err := w.source.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("kafka fetch failed", zap.Error(err))
			if !sleep(ctx, time.Second) {
				return nil
			}
			continue
		}

		if err := w.handle(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("giving up on event after retries",
				zap.ByteString("value", msg.Value), zap.Int64("offset", msg.Offset), zap.Error(err))
		}

		if err := w.source.Commit(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("kafka commit failed", zap.Int64("offset", msg.Offset), zap.Error(err))
		}
	}
}

func (w *Worker) handle(ctx context.Context, msg kafkago.Message) error {
	for attempt := 0; ; attempt++ {
		err := w.handler.HandleVideoUploaded(ctx, msg.Value)
		if err == nil || ctx.Err() != nil || attempt >= len(w.backoff) {
			return err
		}
		w.logger.Warn("event failed, retrying",
			zap.Int("attempt", attempt+1), zap.Duration("backoff", w.backoff[attempt]), zap.Error(err))
		if !sleep(ctx, w.backoff[attempt]) {
			return ctx.Err()
		}
	}
}

// sleep waits for d and reports false if ctx was canceled first.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
