package mediaservice_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Dmitrygosu/stream-engine/internal/app/mediaservice"
	"github.com/Dmitrygosu/stream-engine/internal/model/video"
)

// fakeSource hands out queued messages, then blocks until the context ends.
type fakeSource struct {
	mu        sync.Mutex
	queue     []kafkago.Message
	committed []int64
}

func newSource(values ...string) *fakeSource {
	s := &fakeSource{}
	for i, v := range values {
		s.queue = append(s.queue, kafkago.Message{Offset: int64(i), Value: []byte(v)})
	}
	return s
}

func (s *fakeSource) Fetch(ctx context.Context) (kafkago.Message, error) {
	s.mu.Lock()
	if len(s.queue) > 0 {
		msg := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()
		return msg, nil
	}
	s.mu.Unlock()
	<-ctx.Done()
	return kafkago.Message{}, ctx.Err()
}

func (s *fakeSource) Commit(_ context.Context, msg kafkago.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.committed = append(s.committed, msg.Offset)
	return nil
}

func (s *fakeSource) commits() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int64(nil), s.committed...)
}

type fixture struct {
	repo       *memoryRepo
	producer   *recordingProducer
	transcoder *fakeTranscoder
}

func newFixture() fixture {
	return fixture{repo: newMemoryRepo(), producer: &recordingProducer{}, transcoder: &fakeTranscoder{}}
}

func (f fixture) worker(src *fakeSource) *mediaservice.Worker {
	handler := mediaservice.NewConsumerHandler(f.repo, f.producer, f.transcoder, zap.NewNop())
	return mediaservice.NewWorker(src, handler, zap.NewNop(), []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})
}

// runUntilCommitted runs the worker until want offsets are committed, then
// stops it and checks that Run returned cleanly.
func runUntilCommitted(t *testing.T, w *mediaservice.Worker, src *fakeSource, want int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	require.Eventually(t, func() bool { return len(src.commits()) >= want }, 5*time.Second, time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop after the context was canceled")
	}
}

func TestWorker_PublishesAnUploadedVideo(t *testing.T) {
	f := newFixture()
	v := f.repo.put(video.NewVideo(uuid.New(), "t", ""))
	src := newSource(v.ID.String())

	runUntilCommitted(t, f.worker(src), src, 1)

	assert.Equal(t, video.StatusPublished, f.repo.status(v.ID))
	assert.Equal(t, []video.VideoStatus{video.StatusProcessing, video.StatusPublished}, f.repo.updates)
	require.Len(t, f.producer.events, 1)
	assert.Equal(t, mediaservice.TopicVideoReady, f.producer.events[0].topic)
}

func TestWorker_RetriesTransientFailures(t *testing.T) {
	f := newFixture()
	v := f.repo.put(video.NewVideo(uuid.New(), "t", ""))
	f.repo.updateFailures = 2
	src := newSource(v.ID.String())

	runUntilCommitted(t, f.worker(src), src, 1)

	assert.Equal(t, video.StatusPublished, f.repo.status(v.ID), "the third attempt succeeds")
	assert.Equal(t, []int64{0}, src.commits())
}

func TestWorker_GivesUpAfterRetriesInsteadOfBlockingThePartition(t *testing.T) {
	f := newFixture()
	stuck := f.repo.put(video.NewVideo(uuid.New(), "stuck", ""))
	next := f.repo.put(video.NewVideo(uuid.New(), "next", ""))
	f.repo.updateFailures = 4 // one initial attempt + three retries
	src := newSource(stuck.ID.String(), next.ID.String())

	runUntilCommitted(t, f.worker(src), src, 2)

	assert.Equal(t, video.StatusPending, f.repo.status(stuck.ID))
	assert.Equal(t, video.StatusPublished, f.repo.status(next.ID), "later events are still processed")
}

func TestWorker_MarksTheVideoFailedWhenTranscodingFails(t *testing.T) {
	f := newFixture()
	f.transcoder.err = errors.New("unsupported codec")
	v := f.repo.put(video.NewVideo(uuid.New(), "t", ""))
	src := newSource(v.ID.String())

	runUntilCommitted(t, f.worker(src), src, 1)

	assert.Equal(t, video.StatusFailed, f.repo.status(v.ID), "it must not stay PROCESSING forever")
	assert.Empty(t, f.producer.events)
}

func TestWorker_DropsEventsThatCanNeverSucceed(t *testing.T) {
	f := newFixture()
	src := newSource("not-a-uuid", uuid.NewString())

	runUntilCommitted(t, f.worker(src), src, 2)

	assert.Empty(t, f.repo.updates)
	assert.Zero(t, f.transcoder.callCount())
}

func TestWorker_RedeliveryOfAPublishedVideoOnlyReemitsReady(t *testing.T) {
	f := newFixture()
	v := video.NewVideo(uuid.New(), "t", "")
	v.SetPublished("https://cdn.test/v.m3u8", time.Minute)
	f.repo.put(v)
	src := newSource(v.ID.String())

	runUntilCommitted(t, f.worker(src), src, 1)

	assert.Zero(t, f.transcoder.callCount(), "no second transcoding")
	require.Len(t, f.producer.events, 1)
	assert.Equal(t, mediaservice.TopicVideoReady, f.producer.events[0].topic)
}

func TestWorker_EventInterruptedByShutdownIsNotCommitted(t *testing.T) {
	f := newFixture()
	f.transcoder.block = true
	f.transcoder.started = make(chan struct{})
	v := f.repo.put(video.NewVideo(uuid.New(), "t", ""))
	src := newSource(v.ID.String())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.worker(src).Run(ctx) }()

	<-f.transcoder.started
	cancel()
	require.NoError(t, <-done)
	assert.Empty(t, src.commits(), "Kafka must redeliver the event after a restart")
}
