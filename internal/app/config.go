package app

// Config holds the API server configuration.
type Config struct {
	Port        int
	DatabaseURL string // PostgreSQL connection URL; if empty, uses in-memory SQLite
	Postgres    PostgresConfig
	GitHub      GitHubConfig
	CorsOrigins []string
}

// PostgresConfig holds PostgreSQL connection parameters.
type PostgresConfig struct {
	User     string
	Password string
	HostPort string
	Database string
}

// GitHubConfig holds GitHub API parameters for PR sync.
type GitHubConfig struct {
	Token string
}
