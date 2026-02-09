package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Task struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type Worker struct {
	apiURL       string
	docker       *DockerRunner
	client       *http.Client
	pollInterval time.Duration
}

func New(apiURL string) (*Worker, error) {
	docker, err := NewDockerRunner()
	if err != nil {
		return nil, err
	}

	return &Worker{
		apiURL:       apiURL,
		docker:       docker,
		client:       &http.Client{Timeout: 60 * time.Second},
		pollInterval: 5 * time.Second,
	}, nil
}

func (w *Worker) Close() error {
	return w.docker.Close()
}

func (w *Worker) Run(ctx context.Context) error {
	log.Println("Worker starting...")

	// Ensure agent image exists
	if err := w.docker.EnsureImage(ctx); err != nil {
		return err
	}
	log.Println("Agent image verified")

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down...")
			return ctx.Err()
		default:
		}

		task, err := w.pollForTask(ctx)
		if err != nil {
			log.Printf("Error polling for task: %v", err)
			time.Sleep(w.pollInterval)
			continue
		}

		if task == nil {
			// No task available, continue polling
			continue
		}

		log.Printf("Claimed task %s: %s", task.ID, task.Description)
		w.executeTask(ctx, task)
	}
}

func (w *Worker) pollForTask(ctx context.Context) (*Task, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", w.apiURL+"/api/v1/tasks/poll", nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &task, nil
}

func (w *Worker) executeTask(ctx context.Context, task *Task) {
	// Run the agent
	result := w.docker.RunAgent(ctx, task.ID, task.Description)

	// Print logs locally
	for _, line := range result.Logs {
		log.Printf("[agent] %s", line)
	}

	// Send logs to API server
	if len(result.Logs) > 0 {
		if err := w.sendLogs(ctx, task.ID, result.Logs); err != nil {
			log.Printf("Failed to send logs: %v", err)
		}
	}

	// Report completion
	if result.Error != nil {
		log.Printf("Task %s failed with error: %v", task.ID, result.Error)
		w.completeTask(ctx, task.ID, false, result.Error.Error())
	} else if result.Success {
		log.Printf("Task %s completed successfully", task.ID)
		w.completeTask(ctx, task.ID, true, "")
	} else {
		log.Printf("Task %s failed with exit code %d", task.ID, result.ExitCode)
		w.completeTask(ctx, task.ID, false, fmt.Sprintf("exit code %d", result.ExitCode))
	}
}

func (w *Worker) sendLogs(ctx context.Context, taskID string, logs []string) error {
	body, _ := json.Marshal(map[string][]string{"logs": logs})
	req, err := http.NewRequestWithContext(ctx, "POST", w.apiURL+"/api/v1/tasks/"+taskID+"/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (w *Worker) completeTask(ctx context.Context, taskID string, success bool, errMsg string) error {
	payload := map[string]interface{}{"success": success}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", w.apiURL+"/api/v1/tasks/"+taskID+"/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}
