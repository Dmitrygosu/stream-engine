package authservice

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Dmitrygosu/stream-engine/internal/httpx"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
)

type Handler struct {
	authService *AuthService
}

func NewHandler(authService *AuthService) *Handler {
	return &Handler{authService: authService}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	api := router.Group("/api/v1/auth")
	api.POST("/register", h.register)
	api.POST("/login", h.login)
}

// bcrypt only uses the first 72 bytes of a password and rejects longer ones.
type registerRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Role     string `json:"role" binding:"omitempty,oneof=VIEWER CREATOR"`
}

func (h *Handler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Abort(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	id, err := h.authService.Register(c.Request.Context(), RegisterDTO{
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	})
	switch {
	case errors.Is(err, user.ErrEmailTaken):
		httpx.Abort(c, http.StatusConflict, "email_taken", "email already taken")
	case errors.Is(err, ErrRoleNotAllowed):
		httpx.Abort(c, http.StatusBadRequest, "invalid_request", err.Error())
	case err != nil:
		httpx.Internal(c, err)
	default:
		c.JSON(http.StatusCreated, gin.H{"user_id": id})
	}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,max=72"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Abort(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	token, err := h.authService.Login(c.Request.Context(), LoginDTO{
		Email:    req.Email,
		Password: req.Password,
	})
	switch {
	case errors.Is(err, user.ErrInvalidCredentials):
		httpx.Abort(c, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case err != nil:
		httpx.Internal(c, err)
	default:
		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}
