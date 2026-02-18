package epic

import (
	"context"
	"fmt"
	"time"
)

// TaskCreator creates tasks in the task system when an epic is confirmed.
type TaskCreator interface {
	CreateTaskFromEpic(ctx context.Context, repoID, title, description string, dependsOn, acceptanceCriteria []string, epicID string, ready bool) (string, error)
}

// Store wraps a Repository and adds application-level concerns for epics.
type Store struct {
	repo        Repository
	taskCreator TaskCreator
}

// NewStore creates a new Store backed by the given Repository.
func NewStore(repo Repository, taskCreator TaskCreator) *Store {
	return &Store{repo: repo, taskCreator: taskCreator}
}

// CreateEpic creates a new epic in draft status.
func (s *Store) CreateEpic(ctx context.Context, epic *Epic) error {
	return s.repo.CreateEpic(ctx, epic)
}

// ReadEpic reads an epic by ID.
func (s *Store) ReadEpic(ctx context.Context, id EpicID) (*Epic, error) {
	return s.repo.ReadEpic(ctx, id)
}

// ListEpics returns all epics.
func (s *Store) ListEpics(ctx context.Context) ([]*Epic, error) {
	return s.repo.ListEpics(ctx)
}

// ListEpicsByRepo returns all epics for a given repo.
func (s *Store) ListEpicsByRepo(ctx context.Context, repoID string) ([]*Epic, error) {
	return s.repo.ListEpicsByRepo(ctx, repoID)
}

// StartPlanning transitions an epic from draft to planning status.
func (s *Store) StartPlanning(ctx context.Context, id EpicID, prompt string) error {
	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	if e.Status != StatusDraft && e.Status != StatusReady {
		return fmt.Errorf("epic must be in draft or ready status to start planning")
	}
	e.PlanningPrompt = prompt
	e.Status = StatusPlanning
	e.UpdatedAt = time.Now()
	return s.repo.UpdateEpic(ctx, e)
}

// UpdateProposedTasks updates the proposed tasks for an epic in planning.
func (s *Store) UpdateProposedTasks(ctx context.Context, id EpicID, tasks []ProposedTask) error {
	return s.repo.UpdateProposedTasks(ctx, id, tasks)
}

// AppendSessionLog appends messages to the planning session log.
func (s *Store) AppendSessionLog(ctx context.Context, id EpicID, lines []string) error {
	return s.repo.AppendSessionLog(ctx, id, lines)
}

// FinishPlanning transitions from planning back to draft status, keeping proposed tasks.
func (s *Store) FinishPlanning(ctx context.Context, id EpicID) error {
	return s.repo.UpdateEpicStatus(ctx, id, StatusDraft)
}

// ConfirmEpic creates real tasks from proposed tasks and activates the epic.
func (s *Store) ConfirmEpic(ctx context.Context, id EpicID, notReady bool) error {
	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	if e.Status != StatusDraft && e.Status != StatusReady {
		return fmt.Errorf("epic must be in draft or ready status to confirm")
	}
	if len(e.ProposedTasks) == 0 {
		return fmt.Errorf("epic has no proposed tasks to confirm")
	}

	// Map temp IDs to real task IDs
	tempToReal := make(map[string]string)
	taskIDs := make([]string, 0, len(e.ProposedTasks))

	// Create tasks in dependency order
	for _, pt := range e.ProposedTasks {
		var realDeps []string
		for _, depTempID := range pt.DependsOnTempIDs {
			if realID, ok := tempToReal[depTempID]; ok {
				realDeps = append(realDeps, realID)
			}
		}

		taskID, err := s.taskCreator.CreateTaskFromEpic(
			ctx,
			e.RepoID,
			pt.Title,
			pt.Description,
			realDeps,
			pt.AcceptanceCriteria,
			id.String(),
			!notReady,
		)
		if err != nil {
			return fmt.Errorf("create task %q: %w", pt.Title, err)
		}

		tempToReal[pt.TempID] = taskID
		taskIDs = append(taskIDs, taskID)
	}

	// Store task IDs and update status
	if err := s.repo.SetTaskIDs(ctx, id, taskIDs); err != nil {
		return err
	}

	status := StatusActive
	if notReady {
		status = StatusReady
	}
	e.NotReady = notReady
	return s.repo.UpdateEpicStatus(ctx, id, status)
}

// CloseEpic closes an epic.
func (s *Store) CloseEpic(ctx context.Context, id EpicID) error {
	return s.repo.UpdateEpicStatus(ctx, id, StatusClosed)
}

// DeleteEpic deletes an epic (only if in draft status).
func (s *Store) DeleteEpic(ctx context.Context, id EpicID) error {
	e, err := s.repo.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	if e.Status != StatusDraft {
		return fmt.Errorf("can only delete epics in draft status")
	}
	return s.repo.DeleteEpic(ctx, id)
}
