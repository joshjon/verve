package sqlite

import (
	"github.com/joshjon/kit/tx"

	"github.com/joshjon/verve/internal/sqlite/sqlc"
)

// DB is the interface required by the sqlite package for database access.
// It is satisfied by *sql.DB.
type DB interface {
	sqlc.DBTX
	tx.SQLiteTxer
}
