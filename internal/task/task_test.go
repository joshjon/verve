package task

import (
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	tsk := NewTask("repo_123", "Fix bug", "Fix the login bug", nil, nil, 10.0, false, "sonnet")

	if tsk.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	if tsk.RepoID != "repo_123" {
		t.Errorf("expected RepoID repo_123, got %s", tsk.RepoID)
	}
	if tsk.Title != "Fix bug" {
		t.Errorf("expected Title 'Fix bug', got %s", tsk.Title)
	}
	if tsk.Description != "Fix the login bug" {
		t.Errorf("expected Description 'Fix the login bug', got %s", tsk.Description)
	}
	if tsk.Status != StatusPending {
		t.Errorf("expected status pending, got %s", tsk.Status)
	}
	if tsk.Attempt != 1 {
		t.Errorf("expected Attempt 1, got %d", tsk.Attempt)
	}
	if tsk.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts 5, got %d", tsk.MaxAttempts)
	}
	if tsk.MaxCostUSD != 10.0 {
		t.Errorf("expected MaxCostUSD 10.0, got %f", tsk.MaxCostUSD)
	}
	if tsk.SkipPR != false {
		t.Error("expected SkipPR false")
	}
	if tsk.Model != "sonnet" {
		t.Errorf("expected Model 'sonnet', got %s", tsk.Model)
	}
	if tsk.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if tsk.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestNewTask_NilSlicesBecomEmpty(t *testing.T) {
	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")

	if tsk.DependsOn == nil {
		t.Error("expected DependsOn to be non-nil empty slice")
	}
	if len(tsk.DependsOn) != 0 {
		t.Errorf("expected DependsOn length 0, got %d", len(tsk.DependsOn))
	}
	if tsk.AcceptanceCriteria == nil {
		t.Error("expected AcceptanceCriteria to be non-nil empty slice")
	}
	if len(tsk.AcceptanceCriteria) != 0 {
		t.Errorf("expected AcceptanceCriteria length 0, got %d", len(tsk.AcceptanceCriteria))
	}
}

func TestNewTask_WithDependencies(t *testing.T) {
	deps := []string{"tsk_abc", "tsk_def"}
	criteria := []string{"Tests pass", "No regressions"}
	tsk := NewTask("repo_123", "title", "desc", deps, criteria, 5.0, true, "opus")

	if len(tsk.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(tsk.DependsOn))
	}
	if tsk.DependsOn[0] != "tsk_abc" {
		t.Errorf("expected first dep tsk_abc, got %s", tsk.DependsOn[0])
	}
	if len(tsk.AcceptanceCriteria) != 2 {
		t.Errorf("expected 2 acceptance criteria, got %d", len(tsk.AcceptanceCriteria))
	}
	if !tsk.SkipPR {
		t.Error("expected SkipPR true")
	}
	if tsk.Model != "opus" {
		t.Errorf("expected Model 'opus', got %s", tsk.Model)
	}
}

func TestComputeDuration_NilStartedAt(t *testing.T) {
	tsk := &Task{Status: StatusRunning, StartedAt: nil}
	tsk.ComputeDuration()
	if tsk.DurationMs != nil {
		t.Error("expected DurationMs to be nil when StartedAt is nil")
	}
}

func TestComputeDuration_PendingStatus(t *testing.T) {
	now := time.Now()
	tsk := &Task{Status: StatusPending, StartedAt: &now}
	tsk.ComputeDuration()
	if tsk.DurationMs != nil {
		t.Error("expected DurationMs to be nil for pending status")
	}
}

func TestComputeDuration_RunningStatus(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	tsk := &Task{Status: StatusRunning, StartedAt: &start}
	tsk.ComputeDuration()
	if tsk.DurationMs == nil {
		t.Fatal("expected DurationMs to be set for running status")
	}
	if *tsk.DurationMs < 4000 { // at least ~4 seconds
		t.Errorf("expected duration >= 4000ms, got %d", *tsk.DurationMs)
	}
}

func TestComputeDuration_CompletedStatuses(t *testing.T) {
	statuses := []Status{StatusReview, StatusMerged, StatusClosed, StatusFailed}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			start := time.Now().Add(-10 * time.Second)
			updated := time.Now().Add(-5 * time.Second)
			tsk := &Task{
				Status:    status,
				StartedAt: &start,
				UpdatedAt: updated,
			}
			tsk.ComputeDuration()
			if tsk.DurationMs == nil {
				t.Fatalf("expected DurationMs to be set for %s status", status)
			}
			// Should be approximately 5 seconds (10s start - 5s updated)
			expected := updated.Sub(start).Milliseconds()
			if *tsk.DurationMs != expected {
				t.Errorf("expected duration %dms, got %dms", expected, *tsk.DurationMs)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusReview, "review"},
		{StatusMerged, "merged"},
		{StatusClosed, "closed"},
		{StatusFailed, "failed"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}
