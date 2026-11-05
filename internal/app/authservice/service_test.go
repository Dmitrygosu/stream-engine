package authservice_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/app/authservice"
	"github.com/Dmitrygosu/stream-engine/internal/auth"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
)

// memoryUsers is an in-memory UserRepository keyed by email.
type memoryUsers struct {
	mu    sync.Mutex
	users map[string]user.User
}

func newMemoryUsers() *memoryUsers { return &memoryUsers{users: map[string]user.User{}} }

func (r *memoryUsers) Create(_ context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.users[u.Email]; taken {
		return user.ErrEmailTaken
	}
	r.users[u.Email] = *u
	return nil
}

func (r *memoryUsers) GetByEmail(_ context.Context, email string) (*user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[email]
	if !ok {
		return nil, user.ErrUserNotFound
	}
	return &u, nil
}

func (r *memoryUsers) GetByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, user.ErrUserNotFound
}

func newService() (*authservice.AuthService, *auth.Tokens) {
	tokens := auth.NewTokens([]byte("auth-service-test-secret-32-bytes"), "stream-engine", time.Minute)
	return authservice.NewAuthService(newMemoryUsers(), tokens), tokens
}

func TestRegisterAndLogin_IssueATokenWithTheRole(t *testing.T) {
	svc, tokens := newService()
	ctx := context.Background()

	id, err := svc.Register(ctx, authservice.RegisterDTO{Email: "ann@example.com", Password: "correct horse", Role: "CREATOR"})
	require.NoError(t, err)

	token, err := svc.Login(ctx, authservice.LoginDTO{Email: "ann@example.com", Password: "correct horse"})
	require.NoError(t, err)

	principal, err := tokens.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, auth.Principal{UserID: id, Role: user.RoleCreator}, principal)
}

func TestEmailsAreCaseInsensitive(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.Register(ctx, authservice.RegisterDTO{Email: "  Ann@Example.COM ", Password: "correct horse"})
	require.NoError(t, err)

	_, err = svc.Register(ctx, authservice.RegisterDTO{Email: "ann@example.com", Password: "another one"})
	require.ErrorIs(t, err, user.ErrEmailTaken, "the same address in another case is the same account")

	_, err = svc.Login(ctx, authservice.LoginDTO{Email: "ANN@example.com", Password: "correct horse"})
	require.NoError(t, err)
}

func TestRegister_CannotSelfAssignAdmin(t *testing.T) {
	svc, _ := newService()

	_, err := svc.Register(context.Background(), authservice.RegisterDTO{
		Email: "mallory@example.com", Password: "correct horse", Role: "ADMIN",
	})

	require.ErrorIs(t, err, authservice.ErrRoleNotAllowed)
}

func TestLogin_RejectsWrongCredentials(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	_, err := svc.Register(ctx, authservice.RegisterDTO{Email: "ann@example.com", Password: "correct horse"})
	require.NoError(t, err)

	_, err = svc.Login(ctx, authservice.LoginDTO{Email: "ann@example.com", Password: "wrong horse"})
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
	_, err = svc.Login(ctx, authservice.LoginDTO{Email: "nobody@example.com", Password: "correct horse"})
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
}
