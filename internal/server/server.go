package server

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	echo     *echo.Echo
	store    *Store
	handlers *Handlers
}

func New() *Server {
	store := NewStore()
	handlers := NewHandlers(store)

	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	api := e.Group("/api/v1")

	// Task endpoints
	api.POST("/tasks", handlers.CreateTask)
	api.GET("/tasks", handlers.ListTasks)
	api.GET("/tasks/poll", handlers.PollTask)
	api.GET("/tasks/:id", handlers.GetTask)
	api.POST("/tasks/:id/logs", handlers.AppendLogs)
	api.POST("/tasks/:id/complete", handlers.CompleteTask)

	return &Server{
		echo:     e,
		store:    store,
		handlers: handlers,
	}
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}
