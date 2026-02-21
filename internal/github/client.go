package github

import (
	"context"
	"crypto/tls"
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
// If insecureSkipVerify is true, TLS certificate verification is disabled.
// This may be required in networks with TLS-intercepting proxies.
func NewClient(token string, insecureSkipVerify bool) *Client {
	if token == "" {
		return nil
	}
	httpClient := &http.Client{}
	if insecureSkipVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // intentional for TLS-intercepting proxies
			},
		}
	}
	return &Client{
		token:      token,
		httpClient: httpClient,
	}
}

// ListAccessibleRepos returns repositories the authenticated user has access to.
func (c *Client) ListAccessibleRepos(ctx context.Context) ([]*GitHubRepo, error) {
	url := "https://api.github.com/user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=updated"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return false, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, prURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, checksURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, statusURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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
	checks := make([]IndividualCheck, 0, len(checkRuns)+len(commitStatus.Statuses))

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
		if run.Conclusion != nil && *run.Conclusion == string(CheckStatusFailure) {
			failedNames = append(failedNames, run.Name)
			failedRunIDs = append(failedRunIDs, run.ID)
		}
	}

	for _, s := range commitStatus.Statuses {
		switch s.State {
		case "pending":
			hasPending = true
		case "failure", "error":
			failedNames = append(failedNames, s.Context)
		}
	}

	if len(failedNames) > 0 {
		return &CheckResult{
			Status:           CheckStatusFailure,
			Summary:          fmt.Sprint(failedNames),
			FailedRunIDs:     failedRunIDs,
			FailedNames:      failedNames,
			CheckRunsSkipped: checkRunsSkipped,
			Checks:           checks,
		}, nil
	}
	if hasPending {
		return &CheckResult{Status: CheckStatusPending, CheckRunsSkipped: checkRunsSkipped, Checks: checks}, nil
	}

	// If no check runs and no statuses exist, the repository has no CI configured.
	// Treat as success since there are no checks to wait for.

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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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

// getFailedStepName fetches job metadata from GitHub API and returns the name
// of the first step with conclusion "failure". Returns empty string if no
// failed step is found or on error.
func (c *Client) getFailedStepName(ctx context.Context, owner, repoName string, jobID int64) string {
	jobURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/jobs/%d", owner, repoName, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jobURL, http.NoBody)
	if err != nil {
		return ""
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var job struct {
		Steps []struct {
			Name       string  `json:"name"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
			Number     int     `json:"number"`
		} `json:"steps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return ""
	}

	for _, step := range job.Steps {
		if step.Conclusion != nil && *step.Conclusion == "failure" {
			return step.Name
		}
	}
	return ""
}

// extractStepLogs extracts log lines belonging to a specific step from raw
// GitHub Actions job logs. Steps are delimited by ##[group]<step name> and
// ##[endgroup] markers in the raw log output. Returns only the lines for the
// matching step, or all lines if the step is not found.
func extractStepLogs(rawLog, stepName string) string {
	if stepName == "" {
		return rawLog
	}

	lines := strings.Split(rawLog, "\n")
	var stepLines []string
	inStep := false

	for _, line := range lines {
		// GitHub Actions step markers appear as:
		// 2024-01-01T00:00:00.0000000Z ##[group]Step Name
		if strings.Contains(line, "##[group]") {
			// Extract the step name after ##[group]
			idx := strings.Index(line, "##[group]")
			groupName := line[idx+len("##[group]"):]
			groupName = strings.TrimSpace(groupName)

			if groupName == stepName {
				inStep = true
				continue
			} else if inStep {
				// We've reached the next step, stop collecting
				break
			}
		}
		if strings.Contains(line, "##[endgroup]") && inStep {
			// End of the matched step section; continue because more
			// log lines for the same step may follow after endgroup
			// (endgroup closes a collapsible section within a step,
			// not the step itself).
			continue
		}
		if inStep {
			stepLines = append(stepLines, line)
		}
	}

	// If we found step-specific lines, return them; otherwise fall back to
	// the full log so the caller still gets useful output.
	if len(stepLines) > 0 {
		return strings.Join(stepLines, "\n")
	}
	return rawLog
}

// GetFailedCheckLogs fetches the log output of failed check runs for a PR.
// For each failed job it identifies the exact failed step and returns the last
// 150 lines of that step's logs. Returns a combined, truncated string (~8KB max).
func (c *Client) GetFailedCheckLogs(ctx context.Context, owner, repoName string, prNumber int) (string, error) {
	checkResult, err := c.GetPRCheckStatus(ctx, owner, repoName, prNumber)
	if err != nil {
		return "", fmt.Errorf("get check status: %w", err)
	}
	if len(checkResult.FailedRunIDs) == 0 {
		return "", nil
	}

	const maxTotalBytes = 8192
	const maxLinesPerStep = 150
	parts := make([]string, 0, len(checkResult.FailedRunIDs))
	totalLen := 0

	for i, jobID := range checkResult.FailedRunIDs {
		if totalLen >= maxTotalBytes {
			break
		}

		name := "unknown"
		if i < len(checkResult.FailedNames) {
			name = checkResult.FailedNames[i]
		}

		// Identify the exact failed step within this job.
		failedStep := c.getFailedStepName(ctx, owner, repoName, jobID)

		logURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/jobs/%d/logs", owner, repoName, jobID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, logURL, http.NoBody)
		if err != nil {
			continue
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			parts = append(parts, fmt.Sprintf("=== Failed: %s ===\n(could not fetch logs: HTTP %d)", name, resp.StatusCode))
			continue
		}

		// Extract only the failed step's logs.
		stepLog := extractStepLogs(string(body), failedStep)

		// Take last N lines of the failed step.
		lines := strings.Split(stepLog, "\n")
		if len(lines) > maxLinesPerStep {
			lines = lines[len(lines)-maxLinesPerStep:]
		}
		tail := strings.Join(lines, "\n")

		remaining := maxTotalBytes - totalLen
		if len(tail) > remaining {
			tail = tail[len(tail)-remaining:]
		}

		stepLabel := name
		if failedStep != "" {
			stepLabel = fmt.Sprintf("%s > %s", name, failedStep)
		}
		part := fmt.Sprintf("=== Failed: %s ===\n%s", stepLabel, tail)
		parts = append(parts, part)
		totalLen += len(part)
	}

	return strings.Join(parts, "\n\n"), nil
}

// maxDiffSize is the maximum number of bytes to read from a PR diff response
// to avoid memory issues with very large diffs.
const maxDiffSize = 5 * 1024 * 1024 // 5MB

// GetPRDiff fetches the unified diff for a pull request using the GitHub API.
// It uses the Accept: application/vnd.github.v3.diff header to get raw diff text.
// The response body is limited to maxDiffSize bytes.
func (c *Client) GetPRDiff(ctx context.Context, owner, repo string, prNumber int) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3.diff")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDiffSize))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ClosePR closes an open pull request and returns the head branch name.
func (c *Client) ClosePR(ctx context.Context, owner, repoName string, prNumber int) (headBranch string, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repoName, prNumber)
	body := strings.NewReader(`{"state":"closed"}`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

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

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	return nil
}

// UpdatePR updates the title and body of an existing pull request.
func (c *Client) UpdatePR(ctx context.Context, owner, repoName string, prNumber int, title, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repoName, prNumber)

	payload := map[string]string{
		"title": title,
		"body":  body,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// FindPRForBranch searches for an open PR with the given head branch.
// Returns the PR URL, number, and nil error if found. Returns empty/0 if no PR exists.
func (c *Client) FindPRForBranch(ctx context.Context, owner, repo, branch string) (string, int, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open&per_page=1", owner, repo, owner, branch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", 0, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()

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
