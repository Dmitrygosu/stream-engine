package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dmitrygosu/stream-engine/internal/modules/access/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type AuthService struct {
	repo       UserRepository
	signingKey []byte
}

func New(repo UserRepository, signingKey string) *AuthService {
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
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return uuid.Nil, err
	}
	if existing != nil {
		return uuid.Nil, domain.ErrEmailTaken
	}

	role := domain.Role(dto.Role)
	if role == "" {
		role = domain.RoleViewer
	}

	user, err := domain.NewUser(dto.Email, dto.Password, role)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return uuid.Nil, err
	}

	return user.ID, nil
}

type LoginDTO struct {
	Email    string
	Password string
}

func (s *AuthService) Login(ctx context.Context, dto LoginDTO) (string, error) {
	user, err := s.repo.GetByEmail(ctx, dto.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return "", domain.ErrInvalidCredentials
		}
		return "", err
	}

	if !user.VerifyPassword(dto.Password) {
		return "", domain.ErrInvalidCredentials
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID.String(),
		"role": string(user.Role),
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(s.signingKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}
