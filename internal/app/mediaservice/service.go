package mediaservice

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"github.com/Dmitrygosu/stream-engine/internal/model/video"
	"github.com/google/uuid"
)

type EventProducer interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type VideoRepository interface {
	Create(ctx context.Context, v *video.Video) error
	GetByID(ctx context.Context, id uuid.UUID) (*video.Video, error)
	UpdateStatus(ctx context.Context, v *video.Video) error
}

type StorageService interface {
	GenerateUploadURL(ctx context.Context, objectKey string) (string, error)
}

// LocalStorageService simulates S3 behavior for local dev
type LocalStorageService struct {
	BaseURL string
}

func (s *LocalStorageService) GenerateUploadURL(ctx context.Context, objectKey string) (string, error) {
	u, err := url.Parse(s.BaseURL)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, objectKey)
	return u.String(), nil
}

type MediaService struct {
	repo     VideoRepository
	producer EventProducer
	storage  StorageService
}

func NewMediaService(repo VideoRepository, producer EventProducer, storage StorageService) *MediaService {
	return &MediaService{
		repo:     repo,
		producer: producer,
		storage:  storage,
	}
}

type InitUploadDTO struct {
	OwnerID     uuid.UUID
	Title       string
	Description string
}

func (s *MediaService) InitUpload(ctx context.Context, dto InitUploadDTO) (uuid.UUID, string, error) {
	v := video.NewVideo(dto.OwnerID, dto.Title, dto.Description)

	if err := s.repo.Create(ctx, v); err != nil {
		return uuid.Nil, "", fmt.Errorf("init upload: create video: %w", err)
	}

	objectKey := fmt.Sprintf("videos/%s/raw", v.ID)
	uploadURL, err := s.storage.GenerateUploadURL(ctx, objectKey)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("generate url: %w", err)
	}

	return v.ID, uploadURL, nil
}

func (s *MediaService) UploadFinished(ctx context.Context, videoID uuid.UUID) error {
	v, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}

	if err := s.producer.Publish(ctx, "video.uploaded", []byte(v.ID.String()), []byte(v.ID.String())); err != nil {
		return fmt.Errorf("publish video.uploaded: %w", err)
	}

	return nil
}
