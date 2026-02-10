package server

import (
	"context"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Config holds server configuration
type Config struct {
	GitHubToken string
	GitHubRepo  string
	DatabaseURL string // PostgreSQL connection URL (optional, uses in-memory if empty)
}

type Server struct {
	echo         *echo.Echo
	store        TaskStore
	handlers     *Handlers
	githubClient *GitHubClient
}

// New creates a new server with the given configuration.
// If DatabaseURL is provided, it uses PostgreSQL; otherwise, it uses in-memory storage.
func New(ctx context.Context, cfg Config) (*Server, error) {
	var store TaskStore
	var err error

	if cfg.DatabaseURL != "" {
		log.Println("Using PostgreSQL storage")
		store, err = NewPostgresStore(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("Using in-memory storage (data will not persist)")
		store = NewStore()
	}

	githubClient := NewGitHubClient(cfg.GitHubToken, cfg.GitHubRepo)
	handlers := NewHandlers(store, githubClient)

	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE},
	}))

	// Routes
	api := e.Group("/api/v1")

	// Task endpoints
	api.POST("/tasks", handlers.CreateTask)
	api.GET("/tasks", handlers.ListTasks)
	api.POST("/tasks/sync", handlers.SyncAllTasks)
	api.GET("/tasks/poll", handlers.PollTask)
	api.GET("/tasks/:id", handlers.GetTask)
	api.POST("/tasks/:id/logs", handlers.AppendLogs)
	api.POST("/tasks/:id/complete", handlers.CompleteTask)
	api.POST("/tasks/:id/close", handlers.CloseTask)
	api.POST("/tasks/:id/sync", handlers.SyncTaskStatus)

	return &Server{
		echo:         e,
		store:        store,
		handlers:     handlers,
		githubClient: githubClient,
	}, nil
}

// StartBackgroundSync starts a background goroutine that periodically syncs PR status
func (s *Server) StartBackgroundSync(ctx context.Context, interval time.Duration) {
	if s.githubClient == nil {
		log.Println("GitHub client not configured, background sync disabled")
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.syncAllReviewTasks(ctx)
			}
		}
	}()

	log.Printf("Background PR sync started (interval: %v)", interval)
}

func (s *Server) syncAllReviewTasks(ctx context.Context) {
	tasks := s.store.GetTasksInReview()
	for _, task := range tasks {
		if task.PRNumber > 0 {
			merged, err := s.githubClient.IsPRMerged(ctx, task.PRNumber)
			if err != nil {
				log.Printf("Failed to check PR status for task %s: %v", task.ID, err)
				continue
			}
			if merged {
				s.store.UpdateStatus(task.ID, TaskStatusMerged)
				log.Printf("Task %s PR merged, updated status", task.ID)
			}
		}
	}
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

// Close closes any resources held by the server (like database connections).
func (s *Server) Close() {
	if ps, ok := s.store.(*PostgresStore); ok {
		ps.Close()
	}
}
