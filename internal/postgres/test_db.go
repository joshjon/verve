package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joshjon/kit/pgdb"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/postgres/migrations"
)

// NewTestDB returns a pgxpool.Pool connected to the database specified by
// POSTGRES_URL. If the variable is not set the test is skipped. All
// migrations are applied automatically and the pool is closed when the
// test finishes.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set, skipping postgres test")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	err = pgdb.Migrate(pool, migrations.FS)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}
