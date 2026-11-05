package mediaservice_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/app/mediaservice"
	"github.com/Dmitrygosu/stream-engine/internal/auth"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
	"github.com/Dmitrygosu/stream-engine/internal/model/video"
)

func creator() auth.Principal { return auth.Principal{UserID: uuid.New(), Role: user.RoleCreator} }

func TestInitUpload_CreatesPendingVideoOfTheCaller(t *testing.T) {
	repo := newMemoryRepo()
	svc := mediaservice.NewMediaService(repo, &recordingProducer{}, fakeStorage{})
	owner := creator()

	id, uploadURL, err := svc.InitUpload(context.Background(), mediaservice.InitUploadDTO{OwnerID: owner.UserID, Title: "Intro"})

	require.NoError(t, err)
	assert.Equal(t, "https://storage.test/videos/"+id.String()+"/raw", uploadURL)
	v, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, owner.UserID, v.OwnerID)
	assert.Equal(t, video.StatusPending, v.Status)
}

func TestUploadFinished(t *testing.T) {
	owner := creator()
	pending := func(repo *memoryRepo) *video.Video { return repo.put(video.NewVideo(owner.UserID, "t", "")) }

	t.Run("the owner queues the video once", func(t *testing.T) {
		repo, producer := newMemoryRepo(), &recordingProducer{}
		svc := mediaservice.NewMediaService(repo, producer, fakeStorage{})
		v := pending(repo)

		require.NoError(t, svc.UploadFinished(context.Background(), owner, v.ID))

		require.Len(t, producer.events, 1)
		assert.Equal(t, mediaservice.TopicVideoUploaded, producer.events[0].topic)
		assert.Equal(t, v.ID.String(), producer.events[0].value)
	})

	t.Run("another creator is forbidden", func(t *testing.T) {
		repo, producer := newMemoryRepo(), &recordingProducer{}
		svc := mediaservice.NewMediaService(repo, producer, fakeStorage{})
		v := pending(repo)

		err := svc.UploadFinished(context.Background(), creator(), v.ID)

		require.ErrorIs(t, err, mediaservice.ErrForbidden)
		assert.Empty(t, producer.events)
	})

	t.Run("an admin may finish any upload", func(t *testing.T) {
		repo, producer := newMemoryRepo(), &recordingProducer{}
		svc := mediaservice.NewMediaService(repo, producer, fakeStorage{})
		v := pending(repo)

		admin := auth.Principal{UserID: uuid.New(), Role: user.RoleAdmin}
		require.NoError(t, svc.UploadFinished(context.Background(), admin, v.ID))
		assert.Len(t, producer.events, 1)
	})

	t.Run("a video that is already processed cannot be queued again", func(t *testing.T) {
		repo, producer := newMemoryRepo(), &recordingProducer{}
		svc := mediaservice.NewMediaService(repo, producer, fakeStorage{})
		v := pending(repo)
		v.SetProcessing()
		repo.put(v)

		require.ErrorIs(t, svc.UploadFinished(context.Background(), owner, v.ID), mediaservice.ErrNotPending)
		assert.Empty(t, producer.events)
	})
}

func TestGetVideo_Visibility(t *testing.T) {
	repo := newMemoryRepo()
	svc := mediaservice.NewMediaService(repo, &recordingProducer{}, fakeStorage{})
	owner, stranger := creator(), auth.Principal{UserID: uuid.New(), Role: user.RoleViewer}
	admin := auth.Principal{UserID: uuid.New(), Role: user.RoleAdmin}

	draft := repo.put(video.NewVideo(owner.UserID, "draft", ""))
	public := video.NewVideo(owner.UserID, "public", "")
	public.SetPublished("https://cdn.test/x.m3u8", 0)
	repo.put(public)

	_, err := svc.GetVideo(context.Background(), stranger, draft.ID)
	require.ErrorIs(t, err, video.ErrVideoNotFound, "drafts are hidden from other users")

	for _, caller := range []auth.Principal{owner, admin} {
		_, err := svc.GetVideo(context.Background(), caller, draft.ID)
		require.NoError(t, err)
	}
	_, err = svc.GetVideo(context.Background(), stranger, public.ID)
	require.NoError(t, err, "published videos are visible to everyone")
}
