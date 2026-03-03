package sqlite

import "errors"

// SQLite error codes for constraint violations. These are standard codes
// shared by all SQLite-compatible drivers (modernc.org/sqlite, libsql, etc).
const (
	sqliteConstraint           = 19   // SQLITE_CONSTRAINT
	sqliteConstraintUnique     = 2067 // SQLITE_CONSTRAINT_UNIQUE
	sqliteConstraintPrimaryKey = 1555 // SQLITE_CONSTRAINT_PRIMARYKEY
)

// sqliteErrorCoder matches any SQLite driver error that exposes an integer
// error code (e.g. modernc.org/sqlite.Error, libsql errors).
type sqliteErrorCoder interface {
	Code() int
}

func isSQLiteErrCode(err error, codes ...int) bool {
	var se sqliteErrorCoder
	if errors.As(err, &se) {
		for _, code := range codes {
			if se.Code() == code {
				return true
			}
		}
	}
	return false
}
