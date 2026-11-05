package bootstrap_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/bootstrap"
)

func TestMigrate_IsIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, bootstrap.Migrate(ctx, pool))
	require.NoError(t, bootstrap.Migrate(ctx, pool), "second run must be a no-op")

	var tables int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name IN ('users', 'videos')`).Scan(&tables)
	require.NoError(t, err)
	require.Equal(t, 2, tables)
}
