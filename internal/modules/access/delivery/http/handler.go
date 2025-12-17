package http

import (
	"net/http"

	"github.com/Dmitrygosu/stream-engine/internal/modules/access/domain"
	"github.com/Dmitrygosu/stream-engine/internal/modules/access/service"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	authService *service.AuthService
}

func NewHandler(authService *service.AuthService) *Handler {
	return &Handler{authService: authService}
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/auth")
	{
		api.POST("/register", h.register)
		api.POST("/login", h.login)
	}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role" binding:"omitempty,oneof=VIEWER CREATOR"`
}

func (h *Handler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dto := service.RegisterDTO{
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	}

	id, err := h.authService.Register(c.Request.Context(), dto)
	if err != nil {
		if err == domain.ErrEmailTaken {
			c.JSON(http.StatusConflict, gin.H{"error": "email already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": id})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.authService.Login(c.Request.Context(), service.LoginDTO{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if err == domain.ErrInvalidCredentials || err == domain.ErrUserNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}
