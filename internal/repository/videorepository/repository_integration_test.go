package videorepository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/bootstrap"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
	"github.com/Dmitrygosu/stream-engine/internal/model/video"
	"github.com/Dmitrygosu/stream-engine/internal/repository/userrepository"
	"github.com/Dmitrygosu/stream-engine/internal/repository/videorepository"
)

func TestVideoLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, bootstrap.Migrate(ctx, pool))

	owner, err := user.NewUser(uuid.NewString()+"@example.com", "correct horse", user.RoleCreator)
	require.NoError(t, err)
	require.NoError(t, userrepository.NewUserRepository(pool).Create(ctx, owner))

	repo := videorepository.NewVideoRepository(pool)
	v := video.NewVideo(owner.ID, "Intro", "")
	require.NoError(t, repo.Create(ctx, v))

	v.SetProcessing()
	require.NoError(t, repo.UpdateStatus(ctx, v))
	v.SetPublished("https://cdn.test/intro.m3u8", 95*time.Second)
	require.NoError(t, repo.UpdateStatus(ctx, v))

	got, err := repo.GetByID(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, video.StatusPublished, got.Status)
	assert.Equal(t, "https://cdn.test/intro.m3u8", got.HLSURL)
	assert.Equal(t, 95*time.Second, got.Duration)

	_, err = repo.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, video.ErrVideoNotFound)
	require.ErrorIs(t, repo.UpdateStatus(ctx, video.NewVideo(owner.ID, "ghost", "")), video.ErrVideoNotFound)
}
