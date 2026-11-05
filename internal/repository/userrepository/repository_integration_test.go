package userrepository_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/bootstrap"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
	"github.com/Dmitrygosu/stream-engine/internal/repository/userrepository"
)

func newRepo(t *testing.T) *userrepository.UserRepository {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, bootstrap.Migrate(context.Background(), pool))
	return userrepository.NewUserRepository(pool)
}

// Concurrent registrations with one email: exactly one wins, the others get
// ErrEmailTaken instead of a raw database error.
func TestCreate_DuplicateEmailUnderConcurrency(t *testing.T) {
	repo := newRepo(t)
	email := uuid.NewString() + "@example.com"

	const attempts = 10
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u, err := user.NewUser(email, "correct horse", user.RoleViewer)
			if err == nil {
				err = repo.Create(context.Background(), u)
			}
			errs[i] = err
		}(i)
	}
	wg.Wait()

	created := 0
	for _, err := range errs {
		if err == nil {
			created++
			continue
		}
		assert.ErrorIs(t, err, user.ErrEmailTaken)
	}
	assert.Equal(t, 1, created)

	stored, err := repo.GetByEmail(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, user.RoleViewer, stored.Role)
}
