package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dmitrygosu/stream-engine/internal/modules/media/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VideoRepository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{pool: pool}
}

func (r *VideoRepository) Create(ctx context.Context, v *domain.Video) error {
	const sql = `
		INSERT INTO videos (id, owner_id, title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, sql, v.ID, v.OwnerID, v.Title, v.Description, v.Status, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("video repo: create: %w", err)
	}
	return nil
}

func (r *VideoRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Video, error) {
	const sql = `
		SELECT id, owner_id, title, description, status, original_url, hls_url, duration, created_at, updated_at
		FROM videos
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, sql, id)

	var v domain.Video
	var originalURL, hlsURL *string
	var duration *time.Duration

	if err := row.Scan(&v.ID, &v.OwnerID, &v.Title, &v.Description, &v.Status, &originalURL, &hlsURL, &duration, &v.CreatedAt, &v.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVideoNotFound
		}
		return nil, fmt.Errorf("video repo: get: %w", err)
	}

	if originalURL != nil {
		v.OriginalURL = *originalURL
	}
	if hlsURL != nil {
		v.HLSURL = *hlsURL
	}
	if duration != nil {
		v.Duration = *duration
	}

	return &v, nil
}

func (r *VideoRepository) UpdateStatus(ctx context.Context, v *domain.Video) error {
	const sql = `
		UPDATE videos 
		SET status = $1, hls_url = $2, duration = $3, updated_at = $4
		WHERE id = $5
	`
	tag, err := r.pool.Exec(ctx, sql, v.Status, v.HLSURL, v.Duration, v.UpdatedAt, v.ID)
	if err != nil {
		return fmt.Errorf("video repo: update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVideoNotFound
	}
	return nil
}
