package http

import (
	"net/http"

	"github.com/Dmitrygosu/stream-engine/internal/modules/media/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	mediaService *service.MediaService
}

func NewHandler(mediaService *service.MediaService) *Handler {
	return &Handler{mediaService: mediaService}
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/videos")
	{
		api.POST("/init", h.initUpload)
		api.POST("/finish", h.finishUpload)
	}
}

type initUploadRequest struct {
	OwnerID     string `json:"owner_id" binding:"required,uuid"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

func (h *Handler) initUpload(c *gin.Context) {
	var req initUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ownerID, _ := uuid.Parse(req.OwnerID)
	dto := service.InitUploadDTO{
		OwnerID:     ownerID,
		Title:       req.Title,
		Description: req.Description,
	}

	id, uploadURL, err := h.mediaService.InitUpload(c.Request.Context(), dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"video_id":   id,
		"upload_url": uploadURL,
	})
}

type finishUploadRequest struct {
	VideoID string `json:"video_id" binding:"required,uuid"`
}

func (h *Handler) finishUpload(c *gin.Context) {
	var req finishUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	videoID, _ := uuid.Parse(req.VideoID)

	if err := h.mediaService.UploadFinished(c.Request.Context(), videoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processing_started"})
}
