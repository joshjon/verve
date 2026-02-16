package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// IndividualCheck represents a single CI check with its status and link.
type IndividualCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // "queued", "in_progress", "completed", "pending", "success", "failure", "error"
	Conclusion string `json:"conclusion"` // "success", "failure", "neutral", "cancelled", "skipped", "timed_out", ""
	URL        string `json:"url"`        // Link to the check on GitHub
}

// CheckResult holds the result of a PR check query.
type CheckResult struct {
	Status           CheckStatus
	Summary          string  // Human-readable summary of failures
	FailedRunIDs     []int64 // GitHub Actions job IDs for failed check runs
	FailedNames      []string
	CheckRunsSkipped bool              // True when check runs API returned 403 (fine-grained PAT)
	Checks          []IndividualCheck // Individual check details
}

// GetPRCheckStatus returns the combined check status for a PR's head commit.
// It checks both GitHub Actions (check runs) and legacy commit statuses.
// The check runs endpoint requires the "Checks" permission which is not
// available on fine-grained PATs, so a 403 is handled gracefully by falling
// back to commit statuses only.
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
	// This endpoint requires the "Checks" permission which is NOT available
	// on fine-grained PATs (only GitHub Apps). A 403 is expected and handled
	// by skipping check runs and relying on commit statuses only.
	type checkRun struct {
		ID         int64   `json:"id"`
		Name       string  `json:"name"`
		Status     string  `json:"status"`
		Conclusion *string `json:"conclusion"`
		HTMLURL    string  `json:"html_url"`
	}
	var checkRuns []checkRun
	var checkRunsSkipped bool

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

	if resp.StatusCode == http.StatusOK {
		var body struct {
			CheckRuns []checkRun `json:"check_runs"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, err
		}
		checkRuns = body.CheckRuns
	} else {
		// On 403 or other non-200 responses, checkRuns stays empty — we
		// fall through to commit statuses only.
		checkRunsSkipped = true
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
			Context   string `json:"context"`
			State     string `json:"state"`
			TargetURL string `json:"target_url"`
		} `json:"statuses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commitStatus); err != nil {
		return nil, err
	}

	// Build individual check details.
	var checks []IndividualCheck

	for _, run := range checkRuns {
		conclusion := ""
		if run.Conclusion != nil {
			conclusion = *run.Conclusion
		}
		checks = append(checks, IndividualCheck{
			Name:       run.Name,
			Status:     run.Status,
			Conclusion: conclusion,
			URL:        run.HTMLURL,
		})
	}

	for _, s := range commitStatus.Statuses {
		checks = append(checks, IndividualCheck{
			Name:       s.Context,
			Status:     "completed",
			Conclusion: s.State,
			URL:        s.TargetURL,
		})
	}

	// Combine results: check runs + commit statuses.
	var failedNames []string
	var failedRunIDs []int64
	hasPending := false

	for _, run := range checkRuns {
		if run.Status != "completed" {
			hasPending = true
			continue
		}
		if run.Conclusion != nil && *run.Conclusion == "failure" {
			failedNames = append(failedNames, run.Name)
			failedRunIDs = append(failedRunIDs, run.ID)
		}
	}

	for _, s := range commitStatus.Statuses {
		if s.State == "pending" {
			hasPending = true
		} else if s.State == "failure" || s.State == "error" {
			failedNames = append(failedNames, s.Context)
		}
	}

	if len(failedNames) > 0 {
		return &CheckResult{
			Status:           CheckStatusFailure,
			Summary:          fmt.Sprintf("%s", failedNames),
			FailedRunIDs:     failedRunIDs,
			FailedNames:      failedNames,
			CheckRunsSkipped: checkRunsSkipped,
			Checks:           checks,
		}, nil
	}
	if hasPending {
		return &CheckResult{Status: CheckStatusPending, CheckRunsSkipped: checkRunsSkipped, Checks: checks}, nil
	}

	// If check runs exist and all completed successfully, that's a real success.
	// If no check runs and no statuses exist, GitHub Actions may not have registered
	// runs yet — treat as pending to avoid falsely reporting success.
	if !checkRunsSkipped && len(checkRuns) == 0 && len(commitStatus.Statuses) == 0 {
		return &CheckResult{Status: CheckStatusPending, CheckRunsSkipped: checkRunsSkipped, Checks: checks}, nil
	}

	return &CheckResult{Status: CheckStatusSuccess, CheckRunsSkipped: checkRunsSkipped, Checks: checks}, nil
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

// GetFailedCheckLogs fetches the log output of failed check runs for a PR.
// Returns a combined, truncated string of failed job logs (~4KB max).
func (c *Client) GetFailedCheckLogs(ctx context.Context, owner, repoName string, prNumber int) (string, error) {
	checkResult, err := c.GetPRCheckStatus(ctx, owner, repoName, prNumber)
	if err != nil {
		return "", fmt.Errorf("get check status: %w", err)
	}
	if len(checkResult.FailedRunIDs) == 0 {
		return "", nil
	}

	const maxTotalBytes = 4096
	const maxLinesPerJob = 50
	var parts []string
	totalLen := 0

	for i, jobID := range checkResult.FailedRunIDs {
		if totalLen >= maxTotalBytes {
			break
		}

		name := "unknown"
		if i < len(checkResult.FailedNames) {
			name = checkResult.FailedNames[i]
		}

		logURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/jobs/%d/logs", owner, repoName, jobID)
		req, err := http.NewRequestWithContext(ctx, "GET", logURL, nil)
		if err != nil {
			continue
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			parts = append(parts, fmt.Sprintf("=== Failed: %s ===\n(could not fetch logs: HTTP %d)", name, resp.StatusCode))
			continue
		}

		// Take last N lines
		lines := strings.Split(string(body), "\n")
		if len(lines) > maxLinesPerJob {
			lines = lines[len(lines)-maxLinesPerJob:]
		}
		tail := strings.Join(lines, "\n")

		remaining := maxTotalBytes - totalLen
		if len(tail) > remaining {
			tail = tail[len(tail)-remaining:]
		}

		part := fmt.Sprintf("=== Failed: %s ===\n%s", name, tail)
		parts = append(parts, part)
		totalLen += len(part)
	}

	return strings.Join(parts, "\n\n"), nil
}

// ClosePR closes an open pull request and returns the head branch name.
func (c *Client) ClosePR(ctx context.Context, owner, repoName string, prNumber int) (headBranch string, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repoName, prNumber)
	body := strings.NewReader(`{"state":"closed"}`)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, body)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var pr struct {
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return "", err
	}
	return pr.Head.Ref, nil
}

// DeleteBranch deletes a branch from a repository.
func (c *Client) DeleteBranch(ctx context.Context, owner, repoName, branch string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", owner, repoName, branch)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	return nil
}

// FindPRForBranch searches for an open PR with the given head branch.
// Returns the PR URL, number, and nil error if found. Returns empty/0 if no PR exists.
func (c *Client) FindPRForBranch(ctx context.Context, owner, repo, branch string) (string, int, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open&per_page=1", owner, repo, owner, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var prs []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return "", 0, err
	}

	if len(prs) == 0 {
		return "", 0, nil
	}
	return prs[0].HTMLURL, prs[0].Number, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
}
