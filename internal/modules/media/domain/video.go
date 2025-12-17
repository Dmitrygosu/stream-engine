package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrVideoNotFound = errors.New("video not found")
)

type VideoStatus string

const (
	StatusPending    VideoStatus = "PENDING"
	StatusProcessing VideoStatus = "PROCESSING"
	StatusPublished  VideoStatus = "PUBLISHED"
	StatusFailed     VideoStatus = "FAILED"
)

type Video struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Title       string
	Description string
	Status      VideoStatus
	OriginalURL string // URL where raw file is uploaded
	HLSURL      string // URL for streaming
	Duration    time.Duration
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewVideo(ownerID uuid.UUID, title, description string) *Video {
	return &Video{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

func (v *Video) SetProcessing() {
	v.Status = StatusProcessing
	v.UpdatedAt = time.Now().UTC()
}

func (v *Video) SetPublished(hlsURL string, duration time.Duration) {
	v.Status = StatusPublished
	v.HLSURL = hlsURL
	v.Duration = duration
	v.UpdatedAt = time.Now().UTC()
}

func (v *Video) SetFailed() {
	v.Status = StatusFailed
	v.UpdatedAt = time.Now().UTC()
}
