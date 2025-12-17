package service

import (
	"context"
	"fmt"

	"github.com/Dmitrygosu/stream-engine/internal/modules/media/domain"
	"github.com/google/uuid"
)

// EventProducer interface to decouple from Kafka infrastructure
type EventProducer interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type VideoRepository interface {
	Create(ctx context.Context, v *domain.Video) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Video, error)
	UpdateStatus(ctx context.Context, v *domain.Video) error
}

type MediaService struct {
	repo     VideoRepository
	producer EventProducer
}

func New(repo VideoRepository, producer EventProducer) *MediaService {
	return &MediaService{
		repo:     repo,
		producer: producer,
	}
}

type InitUploadDTO struct {
	OwnerID     uuid.UUID
	Title       string
	Description string
}

func (s *MediaService) InitUpload(ctx context.Context, dto InitUploadDTO) (uuid.UUID, string, error) {
	video := domain.NewVideo(dto.OwnerID, dto.Title, dto.Description)

	if err := s.repo.Create(ctx, video); err != nil {
		return uuid.Nil, "", fmt.Errorf("init upload: create video: %w", err)
	}

	// In a real scenario, we would generate a Presigned URL for S3/MinIO here.
	// For this simulation, we return a mock URL where the client "would" upload.
	uploadURL := fmt.Sprintf("https://storage.stream-engine.local/uploads/%s", video.ID)

	return video.ID, uploadURL, nil
}

func (s *MediaService) UploadFinished(ctx context.Context, videoID uuid.UUID) error {
	video, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}

	// Publish event "VideoUploaded" so the Worker can pick it up
	// Key: VideoID, Value: JSON (omitted for brevity in this MVP step, usually a struct)
	if err := s.producer.Publish(ctx, "video.uploaded", []byte(video.ID.String()), []byte(video.ID.String())); err != nil {
		return fmt.Errorf("publish video.uploaded: %w", err)
	}

	return nil
}
