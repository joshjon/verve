package worker

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const DefaultAgentImage = "verve-agent:latest"

type DockerRunner struct {
	client     *client.Client
	agentImage string
}

func NewDockerRunner(agentImage string) (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	if agentImage == "" {
		agentImage = DefaultAgentImage
	}
	return &DockerRunner{client: cli, agentImage: agentImage}, nil
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
			if tag == d.agentImage {
				return nil
			}
		}
	}

	return fmt.Errorf("agent image %s not found - run 'make build-agent' or 'docker pull %s'", d.agentImage, d.agentImage)
}

// AgentImage returns the configured agent image name
func (d *DockerRunner) AgentImage() string {
	return d.agentImage
}

type RunResult struct {
	Success  bool
	ExitCode int
	Error    error
}

// AgentConfig holds the configuration for running an agent
type AgentConfig struct {
	TaskID          string
	TaskDescription string
	GitHubToken     string
	GitHubRepo      string
	AnthropicAPIKey string
	ClaudeModel     string
}

// LogCallback is called for each log line from the container
type LogCallback func(line string)

// RunAgent runs the agent container and streams logs via the callback in real-time.
// The callback is called from a separate goroutine as logs arrive.
func (d *DockerRunner) RunAgent(ctx context.Context, cfg AgentConfig, onLog LogCallback) RunResult {
	// Create container with all required environment variables
	resp, err := d.client.ContainerCreate(ctx,
		&container.Config{
			Image: d.agentImage,
			Env: []string{
				"TASK_ID=" + cfg.TaskID,
				"TASK_DESCRIPTION=" + cfg.TaskDescription,
				"GITHUB_TOKEN=" + cfg.GitHubToken,
				"GITHUB_REPO=" + cfg.GitHubRepo,
				"ANTHROPIC_API_KEY=" + cfg.AnthropicAPIKey,
				"CLAUDE_MODEL=" + cfg.ClaudeModel,
			},
		},
		&container.HostConfig{
			AutoRemove: false, // We'll remove it manually after getting logs
		},
		nil, nil,
		"verve-agent-"+cfg.TaskID,
	)
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to create container: %w", err)}
	}
	containerID := resp.ID

	// Ensure cleanup
	defer func() {
		// Remove container
		if err := d.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("Warning: failed to remove container %s: %v", containerID, err)
		}
	}()

	// Start container
	if err := d.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return RunResult{Error: fmt.Errorf("failed to start container: %w", err)}
	}

	// Attach to logs with Follow=true for real-time streaming
	logReader, err := d.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true, // Stream logs in real-time
		Timestamps: false,
	})
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to attach logs: %w", err)}
	}

	// Stream logs in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logReader.Close()
		d.streamLogs(logReader, onLog)
	}()

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

	// Wait for log streaming to complete
	wg.Wait()

	return RunResult{
		Success:  exitCode == 0,
		ExitCode: int(exitCode),
	}
}

// streamLogs reads from the Docker multiplexed log stream and calls the callback for each line
func (d *DockerRunner) streamLogs(reader io.Reader, onLog LogCallback) {
	// Create a pipe to demultiplex Docker's stream format
	stdoutPipeR, stdoutPipeW := io.Pipe()
	stderrPipeR, stderrPipeW := io.Pipe()

	// Demultiplex in a goroutine
	go func() {
		defer stdoutPipeW.Close()
		defer stderrPipeW.Close()
		_, err := stdcopy.StdCopy(stdoutPipeW, stderrPipeW, reader)
		if err != nil && err != io.EOF {
			log.Printf("Warning: error demultiplexing logs: %v", err)
		}
	}()

	// Read stdout and stderr concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		readLines(stdoutPipeR, func(line string) {
			onLog(line)
		})
	}()

	// Read stderr
	go func() {
		defer wg.Done()
		readLines(stderrPipeR, func(line string) {
			onLog("[stderr] " + line)
		})
	}()

	wg.Wait()
}

// readLines reads lines from a reader and calls the callback for each line
func readLines(reader io.Reader, onLine func(string)) {
	buf := make([]byte, 4096)
	var lineBuf []byte

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Process the data
			data := buf[:n]
			for _, b := range data {
				if b == '\n' {
					if len(lineBuf) > 0 {
						onLine(string(lineBuf))
						lineBuf = lineBuf[:0]
					}
				} else {
					lineBuf = append(lineBuf, b)
				}
			}
		}
		if err != nil {
			// Flush any remaining data
			if len(lineBuf) > 0 {
				onLine(string(lineBuf))
			}
			break
		}
	}
}
