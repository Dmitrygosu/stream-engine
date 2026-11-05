package mediaservice

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Dmitrygosu/stream-engine/internal/auth"
	"github.com/Dmitrygosu/stream-engine/internal/httpx"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
	"github.com/Dmitrygosu/stream-engine/internal/model/video"
)

// Service is the media application layer used by the HTTP handlers.
type Service interface {
	InitUpload(ctx context.Context, dto InitUploadDTO) (uuid.UUID, string, error)
	UploadFinished(ctx context.Context, caller auth.Principal, videoID uuid.UUID) error
	GetVideo(ctx context.Context, caller auth.Principal, videoID uuid.UUID) (*video.Video, error)
}

type Handler struct {
	mediaService Service
}

func NewHandler(mediaService Service) *Handler {
	return &Handler{mediaService: mediaService}
}

// RegisterRoutes mounts the media API. Every route needs a valid token;
// uploading is limited to creators and admins.
func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	api := router.Group("/api/v1/videos", requireAuth)
	uploaders := auth.RequireRole(user.RoleCreator, user.RoleAdmin)

	api.POST("/init", uploaders, h.initUpload)
	api.POST("/finish", uploaders, h.finishUpload)
	api.GET("/:id", h.getVideo)
}

type initUploadRequest struct {
	Title       string `json:"title" binding:"required,max=255"`
	Description string `json:"description" binding:"max=5000"`
}

func (h *Handler) initUpload(c *gin.Context) {
	var req initUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Abort(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	caller, _ := auth.PrincipalFrom(c)

	id, uploadURL, err := h.mediaService.InitUpload(c.Request.Context(), InitUploadDTO{
		OwnerID:     caller.UserID,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"video_id":   id,
		"upload_url": uploadURL,
	})
}

type finishUploadRequest struct {
	VideoID uuid.UUID `json:"video_id" binding:"required"`
}

func (h *Handler) finishUpload(c *gin.Context) {
	var req finishUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Abort(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	caller, _ := auth.PrincipalFrom(c)

	if err := h.mediaService.UploadFinished(c.Request.Context(), caller, req.VideoID); err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "processing_started"})
}

type videoResponse struct {
	ID              uuid.UUID `json:"id"`
	OwnerID         uuid.UUID `json:"owner_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	HLSURL          string    `json:"hls_url,omitempty"`
	DurationSeconds int64     `json:"duration_seconds,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (h *Handler) getVideo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Abort(c, http.StatusBadRequest, "invalid_request", "video id must be a UUID")
		return
	}
	caller, _ := auth.PrincipalFrom(c)

	v, err := h.mediaService.GetVideo(c.Request.Context(), caller, id)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, videoResponse{
		ID:              v.ID,
		OwnerID:         v.OwnerID,
		Title:           v.Title,
		Description:     v.Description,
		Status:          string(v.Status),
		HLSURL:          v.HLSURL,
		DurationSeconds: int64(v.Duration / time.Second),
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	})
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, video.ErrVideoNotFound):
		httpx.Abort(c, http.StatusNotFound, "video_not_found", err.Error())
	case errors.Is(err, ErrForbidden):
		httpx.Abort(c, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, ErrNotPending):
		httpx.Abort(c, http.StatusConflict, "upload_already_finished", err.Error())
	default:
		httpx.Internal(c, err)
	}
}
