package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Client handles GitHub API interactions.
type Client struct {
	token      string
	httpClient *http.Client
	owner      string
	repo       string
}

// NewClient creates a new GitHub API client.
// Returns nil if token or repoFullName is empty.
func NewClient(token, repoFullName string) *Client {
	if token == "" || repoFullName == "" {
		return nil
	}

	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return nil
	}

	return &Client{
		token:      token,
		httpClient: &http.Client{},
		owner:      parts[0],
		repo:       parts[1],
	}
}

// IsPRMerged checks if a PR has been merged.
func (c *Client) IsPRMerged(ctx context.Context, prNumber int) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", c.owner, c.repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
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
