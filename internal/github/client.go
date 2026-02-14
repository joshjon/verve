package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GitHubRepo represents a repository returned by the GitHub API.
type GitHubRepo struct {
	FullName    string `json:"full_name"`
	Owner       string `json:"owner_login"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	HTMLURL     string `json:"html_url"`
}

// Client handles GitHub API interactions.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new GitHub API client.
// Returns nil if token is empty.
func NewClient(token string) *Client {
	if token == "" {
		return nil
	}
	return &Client{
		token:      token,
		httpClient: &http.Client{},
	}
}

// ListAccessibleRepos returns repositories the authenticated user has access to.
func (c *Client) ListAccessibleRepos(ctx context.Context) ([]*GitHubRepo, error) {
	url := "https://api.github.com/user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=updated"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var apiRepos []struct {
		FullName    string `json:"full_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		HTMLURL     string `json:"html_url"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiRepos); err != nil {
		return nil, err
	}

	repos := make([]*GitHubRepo, len(apiRepos))
	for i, r := range apiRepos {
		repos[i] = &GitHubRepo{
			FullName:    r.FullName,
			Owner:       r.Owner.Login,
			Name:        r.Name,
			Description: r.Description,
			Private:     r.Private,
			HTMLURL:     r.HTMLURL,
		}
	}
	return repos, nil
}

// IsPRMerged checks if a PR has been merged for the given owner/repo.
func (c *Client) IsPRMerged(ctx context.Context, owner, repo string, prNumber int) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	c.setHeaders(req)

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

// CheckStatus represents the combined CI check result for a PR.
type CheckStatus string

const (
	CheckStatusPending CheckStatus = "pending"
	CheckStatusSuccess CheckStatus = "success"
	CheckStatusFailure CheckStatus = "failure"
)

// CheckResult holds the result of a PR check query.
type CheckResult struct {
	Status  CheckStatus
	Summary string // Human-readable summary of failures
}

// GetPRCheckStatus returns the combined check status for a PR's head commit.
// It checks both GitHub Actions (check runs) and legacy commit statuses.
func (c *Client) GetPRCheckStatus(ctx context.Context, owner, repo string, prNumber int) (*CheckResult, error) {
	// Step 1: Get the PR to find the head SHA.
	prURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", prURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for PR", resp.StatusCode)
	}

	var pr struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	headSHA := pr.Head.SHA

	// Step 2: Get check runs for the head SHA (GitHub Actions).
	checksURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s/check-runs", owner, repo, headSHA)
	req, err = http.NewRequestWithContext(ctx, "GET", checksURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for check runs", resp.StatusCode)
	}

	var checkRuns struct {
		TotalCount int `json:"total_count"`
		CheckRuns  []struct {
			Name       string  `json:"name"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&checkRuns); err != nil {
		return nil, err
	}

	// Step 3: Get combined commit status (legacy status API).
	statusURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s/status", owner, repo, headSHA)
	req, err = http.NewRequestWithContext(ctx, "GET", statusURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for commit status", resp.StatusCode)
	}

	var commitStatus struct {
		State    string `json:"state"` // "success", "failure", "pending"
		Statuses []struct {
			Context string `json:"context"`
			State   string `json:"state"`
		} `json:"statuses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commitStatus); err != nil {
		return nil, err
	}

	// Combine results: check runs + commit statuses.
	var failed []string
	hasPending := false

	for _, run := range checkRuns.CheckRuns {
		if run.Status != "completed" {
			hasPending = true
			continue
		}
		if run.Conclusion != nil && *run.Conclusion == "failure" {
			failed = append(failed, run.Name)
		}
	}

	for _, s := range commitStatus.Statuses {
		if s.State == "pending" {
			hasPending = true
		} else if s.State == "failure" || s.State == "error" {
			failed = append(failed, s.Context)
		}
	}

	if len(failed) > 0 {
		return &CheckResult{
			Status:  CheckStatusFailure,
			Summary: fmt.Sprintf("%s", failed),
		}, nil
	}
	if hasPending {
		return &CheckResult{Status: CheckStatusPending}, nil
	}

	// If there are no check runs and no statuses, treat as success (no CI configured).
	return &CheckResult{Status: CheckStatusSuccess}, nil
}

// PRMergeability holds the mergeability state of a PR.
type PRMergeability struct {
	Mergeable      *bool  // nil = not yet computed by GitHub
	MergeableState string // "clean", "dirty", "blocked", "behind", "unstable"
	HasConflicts   bool   // true when mergeable_state == "dirty"
}

// GetPRMergeability checks whether a PR has merge conflicts.
func (c *Client) GetPRMergeability(ctx context.Context, owner, repo string, prNumber int) (*PRMergeability, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var pr struct {
		Mergeable      *bool  `json:"mergeable"`
		MergeableState string `json:"mergeable_state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}

	return &PRMergeability{
		Mergeable:      pr.Mergeable,
		MergeableState: pr.MergeableState,
		HasConflicts:   pr.MergeableState == "dirty",
	}, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
}
