package app

import "time"

// Config holds the API server configuration.
type Config struct {
	Port                       int
	UI                         bool
	Postgres                   PostgresConfig // If empty, uses SQLite
	SQLiteDir                  string         // Directory for SQLite DB file; if empty, uses in-memory
	EncryptionKey              string         // Hex-encoded 32-byte key for encrypting secrets at rest
	GitHubInsecureSkipVerify   bool           // Disable TLS certificate verification for GitHub API calls
	CorsOrigins                []string
	TaskTimeout                time.Duration // How long before a running task with no heartbeat is considered stale (default: 5m)
	LogRetention               time.Duration // How long to keep task logs before deleting them (0 = keep forever)
}

// PostgresConfig holds PostgreSQL connection parameters.
type PostgresConfig struct {
	User     string
	Password string
	HostPort string
	Database string
}

// IsSet returns true if Postgres connection parameters are configured.
func (c PostgresConfig) IsSet() bool {
	return c.HostPort != ""
}
