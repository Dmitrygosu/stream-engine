package authservice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Dmitrygosu/stream-engine/internal/model/user"
)

// ErrRoleNotAllowed is returned when a user tries to register with a role that
// cannot be self-assigned.
var ErrRoleNotAllowed = errors.New("role cannot be chosen at registration")

type UserRepository interface {
	Create(ctx context.Context, u *user.User) error
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*user.User, error)
}

// TokenIssuer signs access tokens for authenticated users.
type TokenIssuer interface {
	Issue(userID uuid.UUID, role user.Role) (string, error)
}

type AuthService struct {
	repo   UserRepository
	tokens TokenIssuer
	// dummyHash is compared against when the email is unknown, so that a
	// login takes the same time whether or not the account exists.
	dummyHash []byte
}

func NewAuthService(repo UserRepository, tokens TokenIssuer) *AuthService {
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("dummy password for timing"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("authservice: generate dummy hash: %v", err))
	}
	return &AuthService{repo: repo, tokens: tokens, dummyHash: dummyHash}
}

// normalizeEmail makes addresses that differ only in case or surrounding
// spaces refer to the same account.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type RegisterDTO struct {
	Email    string
	Password string
	Role     string
}

func (s *AuthService) Register(ctx context.Context, dto RegisterDTO) (uuid.UUID, error) {
	role := user.Role(dto.Role)
	if role == "" {
		role = user.RoleViewer
	}
	// Admins are appointed, never self-registered.
	if role != user.RoleViewer && role != user.RoleCreator {
		return uuid.Nil, ErrRoleNotAllowed
	}

	newUser, err := user.NewUser(normalizeEmail(dto.Email), dto.Password, role)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	// The unique index on email makes concurrent registrations safe: the
	// repository reports a duplicate as user.ErrEmailTaken.
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
	u, err := s.repo.GetByEmail(ctx, normalizeEmail(dto.Email))
	if errors.Is(err, user.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(dto.Password))
		return "", user.ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}

	if !u.VerifyPassword(dto.Password) {
		return "", user.ErrInvalidCredentials
	}
	return s.tokens.Issue(u.ID, u.Role)
}
