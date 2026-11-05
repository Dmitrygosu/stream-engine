package mediaservice_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Dmitrygosu/stream-engine/internal/model/video"
)

// memoryRepo is an in-memory VideoRepository.
type memoryRepo struct {
	mu        sync.Mutex
	videos    map[uuid.UUID]video.Video
	getErr    error
	updateErr error
	// updateFailures makes the next N UpdateStatus calls fail, like a database outage.
	updateFailures int
	updates        []video.VideoStatus
}

func newMemoryRepo() *memoryRepo { return &memoryRepo{videos: map[uuid.UUID]video.Video{}} }

func (r *memoryRepo) Create(_ context.Context, v *video.Video) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.videos[v.ID] = *v
	return nil
}

func (r *memoryRepo) GetByID(_ context.Context, id uuid.UUID) (*video.Video, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	v, ok := r.videos[id]
	if !ok {
		return nil, video.ErrVideoNotFound
	}
	return &v, nil
}

func (r *memoryRepo) UpdateStatus(_ context.Context, v *video.Video) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	if r.updateFailures > 0 {
		r.updateFailures--
		return errors.New("connection reset by peer")
	}
	r.videos[v.ID] = *v
	r.updates = append(r.updates, v.Status)
	return nil
}

func (r *memoryRepo) put(v *video.Video) *video.Video {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.videos[v.ID] = *v
	return v
}

func (r *memoryRepo) status(id uuid.UUID) video.VideoStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.videos[id].Status
}

type published struct {
	topic string
	value string
}

// recordingProducer records published events.
type recordingProducer struct {
	mu     sync.Mutex
	events []published
	err    error
}

func (p *recordingProducer) Publish(_ context.Context, topic string, _, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, published{topic: topic, value: string(value)})
	return nil
}

type fakeStorage struct{}

func (fakeStorage) GenerateUploadURL(_ context.Context, key string) (string, error) {
	return "https://storage.test/" + key, nil
}

// fakeTranscoder returns preset results; with block set it waits for the
// context to be canceled, like a long job interrupted by shutdown.
type fakeTranscoder struct {
	mu      sync.Mutex
	err     error
	calls   int
	block   bool
	started chan struct{}
}

func (t *fakeTranscoder) Transcode(ctx context.Context, fileID string) (string, time.Duration, error) {
	t.mu.Lock()
	t.calls++
	block, started := t.block, t.started
	t.mu.Unlock()
	if started != nil {
		close(started)
	}
	if block {
		<-ctx.Done()
		return "", 0, ctx.Err()
	}
	if t.err != nil {
		return "", 0, t.err
	}
	return "https://cdn.test/" + fileID + "/master.m3u8", 90 * time.Second, nil
}

func (t *fakeTranscoder) callCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}
