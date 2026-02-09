package worker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const AgentImage = "verve-agent:latest"

type DockerRunner struct {
	client *client.Client
}

func NewDockerRunner() (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerRunner{client: cli}, nil
}

func (d *DockerRunner) Close() error {
	return d.client.Close()
}

// EnsureImage checks if the agent image exists locally
func (d *DockerRunner) EnsureImage(ctx context.Context) error {
	images, err := d.client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == AgentImage {
				return nil
			}
		}
	}

	return fmt.Errorf("agent image %s not found - run 'make build-agent' first", AgentImage)
}

type RunResult struct {
	Success  bool
	ExitCode int
	Error    error
	Logs     []string
}

// RunAgent runs the agent container and returns the result with logs
func (d *DockerRunner) RunAgent(ctx context.Context, taskID, description string) RunResult {
	// Create container
	resp, err := d.client.ContainerCreate(ctx,
		&container.Config{
			Image: AgentImage,
			Env: []string{
				"TASK_ID=" + taskID,
				"TASK_DESCRIPTION=" + description,
			},
		},
		&container.HostConfig{
			AutoRemove: false, // We'll remove it manually after getting logs
		},
		nil, nil,
		"verve-agent-"+taskID,
	)
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to create container: %w", err)}
	}
	containerID := resp.ID

	// Ensure cleanup
	defer func() {
		// Remove container
		if err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("Warning: failed to remove container %s: %v", containerID, err)
		}
	}()

	// Start container
	if err := d.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return RunResult{Error: fmt.Errorf("failed to start container: %w", err)}
	}

	// Wait for container to finish
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var exitCode int64
	select {
	case err := <-errCh:
		return RunResult{Error: fmt.Errorf("error waiting for container: %w", err)}
	case status := <-statusCh:
		exitCode = status.StatusCode
	case <-ctx.Done():
		return RunResult{Error: ctx.Err()}
	}

	// Get logs after container finishes
	logReader, err := d.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Timestamps: false,
	})
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to get logs: %w", err)}
	}
	defer logReader.Close()

	logs := d.collectLogs(logReader)

	return RunResult{
		Success:  exitCode == 0,
		ExitCode: int(exitCode),
		Logs:     logs,
	}
}

func (d *DockerRunner) collectLogs(reader io.Reader) []string {
	// Docker uses a multiplexed stream format with 8-byte headers
	// Use stdcopy to demultiplex stdout and stderr
	var stdout, stderr bytes.Buffer
	_, err := stdcopy.StdCopy(&stdout, &stderr, reader)
	if err != nil {
		log.Printf("Warning: error reading container logs: %v", err)
	}

	var logs []string

	// Process stdout
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			logs = append(logs, line)
		}
	}

	// Process stderr
	scanner = bufio.NewScanner(&stderr)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			logs = append(logs, "[stderr] "+line)
		}
	}

	return logs
}
