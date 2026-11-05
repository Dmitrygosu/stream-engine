package mediaservice_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/app/mediaservice"
	"github.com/Dmitrygosu/stream-engine/internal/auth"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
	"github.com/Dmitrygosu/stream-engine/internal/model/video"
)

type api struct {
	router *gin.Engine
	tokens *auth.Tokens
	repo   *memoryRepo
}

func newAPI(t *testing.T) api {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tokens := auth.NewTokens([]byte("handler-test-secret-of-32-bytes!!"), "stream-engine", time.Minute)
	repo := newMemoryRepo()
	r := gin.New()
	svc := mediaservice.NewMediaService(repo, &recordingProducer{}, fakeStorage{})
	mediaservice.NewHandler(svc).RegisterRoutes(r, auth.RequireAuth(tokens))
	return api{router: r, tokens: tokens, repo: repo}
}

func (a api) call(t *testing.T, caller auth.Principal, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := a.tokens.Issue(caller.UserID, caller.Role)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

func TestAPI_UploadRequiresTheCreatorRole(t *testing.T) {
	a := newAPI(t)
	viewer := auth.Principal{UserID: uuid.New(), Role: user.RoleViewer}

	rec := a.call(t, viewer, http.MethodPost, "/api/v1/videos/init", `{"title":"cats"}`)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAPI_OwnerComesFromTheToken(t *testing.T) {
	a := newAPI(t)
	owner := creator()

	rec := a.call(t, owner, http.MethodPost, "/api/v1/videos/init",
		`{"title":"cats","owner_id":"`+uuid.NewString()+`"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		VideoID uuid.UUID `json:"video_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	v, err := a.repo.GetByID(t.Context(), body.VideoID)
	require.NoError(t, err)
	assert.Equal(t, owner.UserID, v.OwnerID, "owner_id from the body must be ignored")
}

func TestAPI_FinishUpload(t *testing.T) {
	a := newAPI(t)
	owner := creator()
	v := a.repo.put(video.NewVideo(owner.UserID, "t", ""))
	body := `{"video_id":"` + v.ID.String() + `"}`

	assert.Equal(t, http.StatusForbidden, a.call(t, creator(), http.MethodPost, "/api/v1/videos/finish", body).Code)
	assert.Equal(t, http.StatusAccepted, a.call(t, owner, http.MethodPost, "/api/v1/videos/finish", body).Code)
	unknown := `{"video_id":"` + uuid.NewString() + `"}`
	assert.Equal(t, http.StatusNotFound, a.call(t, owner, http.MethodPost, "/api/v1/videos/finish", unknown).Code)
	assert.Equal(t, http.StatusBadRequest, a.call(t, owner, http.MethodPost, "/api/v1/videos/finish", `{}`).Code)
}

func TestAPI_GetVideo(t *testing.T) {
	a := newAPI(t)
	owner := creator()
	v := video.NewVideo(owner.UserID, "t", "d")
	v.SetPublished("https://cdn.test/v.m3u8", 95*time.Second)
	a.repo.put(v)

	rec := a.call(t, auth.Principal{UserID: uuid.New(), Role: user.RoleViewer}, http.MethodGet, "/api/v1/videos/"+v.ID.String(), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"PUBLISHED"`)
	assert.Contains(t, rec.Body.String(), `"duration_seconds":95`)
}
