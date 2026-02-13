package postgres

import "verve/internal/postgres/sqlc"

// DB is the database interface required by the postgres package.
// It is satisfied by *pgxpool.Pool.
type DB = sqlc.DBTX
