package authservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dmitrygosu/stream-engine/internal/model/user"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, u *user.User) error
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*user.User, error)
}

type AuthService struct {
	repo       UserRepository
	signingKey []byte
}

func NewAuthService(repo UserRepository, signingKey string) *AuthService {
	return &AuthService{
		repo:       repo,
		signingKey: []byte(signingKey),
	}
}

type RegisterDTO struct {
	Email    string
	Password string
	Role     string
}

func (s *AuthService) Register(ctx context.Context, dto RegisterDTO) (uuid.UUID, error) {
	existing, err := s.repo.GetByEmail(ctx, dto.Email)
	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		return uuid.Nil, err
	}
	if existing != nil {
		return uuid.Nil, user.ErrEmailTaken
	}

	role := user.Role(dto.Role)
	if role == "" {
		role = user.RoleViewer
	}

	newUser, err := user.NewUser(dto.Email, dto.Password, role)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}

	if err := s.repo.Create(ctx, newUser); err != nil {
		return uuid.Nil, err
	}

	return newUser.ID, nil
}

type LoginDTO struct {
	Email    string
	Password string
}

func (s *AuthService) Login(ctx context.Context, dto LoginDTO) (string, error) {
	u, err := s.repo.GetByEmail(ctx, dto.Email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return "", user.ErrInvalidCredentials
		}
		return "", err
	}

	if !u.VerifyPassword(dto.Password) {
		return "", user.ErrInvalidCredentials
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  u.ID.String(),
		"role": string(u.Role),
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(s.signingKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}
