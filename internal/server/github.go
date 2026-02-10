package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GitHubClient handles GitHub API interactions
type GitHubClient struct {
	token      string
	httpClient *http.Client
	owner      string
	repo       string
}

// NewGitHubClient creates a new GitHub API client.
// Returns nil if token or repoFullName is empty.
func NewGitHubClient(token, repoFullName string) *GitHubClient {
	if token == "" || repoFullName == "" {
		return nil
	}

	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return nil
	}

	return &GitHubClient{
		token:      token,
		httpClient: &http.Client{},
		owner:      parts[0],
		repo:       parts[1],
	}
}

// IsPRMerged checks if a PR has been merged
func (g *GitHubClient) IsPRMerged(ctx context.Context, prNumber int) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", g.owner, g.repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var pr struct {
		Merged bool   `json:"merged"`
		State  string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return false, err
	}

	return pr.Merged, nil
}
