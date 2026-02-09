package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"verve/internal/worker"
)

func main() {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	log.Printf("Connecting to API server at %s", apiURL)

	w, err := worker.New(apiURL)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}
	defer w.Close()

	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal")
		cancel()
	}()

	if err := w.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Worker error: %v", err)
	}
}
